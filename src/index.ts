import {
	Plugin, 
	Setting, 
	showMessage, 
	fetchPost,
	fetchSyncPost,
	
	// Menu,
	// getFrontend, 
	// IOperation, 
	// Dialog, 
	// openSetting
} from "siyuan";

import JSZip = require("jszip");

import "./index.scss";

const STORAGE_KEY = "custompic-config";

interface ServerEntry {
	id: string;
	name: string;
	baseURL: string;
	authHeader: string;
	/** 未设置或为 true 时参与上传；false 时跳过 */
	enabled?: boolean;
}

/** 默认包含：常用图片 + 常用视频（可含 * 前缀，逗号/空格分隔） */
const DEFAULT_FILE_SUFFIXES =
	"*.jpg, *.jpeg, *.png, *.gif, *.webp, *.bmp, *.tif, *.tiff, *.svg, " +
	"*.heic, *.avif, *.mp4, *.webm, *.mov, *.mkv, *.avi, *.m4v, *.mpeg, *.mpg, *.wmv, *.flv, *.3gp";

export default class CustompicUploader extends Plugin {

	private config: any = {};
	private uploadedFiles: Set<string> = new Set();
	private supportedSuffixes: string[] = [];

	private currentPageId: string = ""; //当前页面id
	private imageMap: Map<string, string> = new Map(); // 新增图片映射表;
	// key: 资源路径（/data/assets/..），value: 命中的 blockId 列表
	private blockAssetMap: Map<string, Set<string>> = new Map();


	async onload() {
		await this.loadData(STORAGE_KEY);
		this.config = this.data[STORAGE_KEY] || {};
		this.migrateServerConfig();

		this.addTopBar({
			icon: "iconUpload",
			title: this.i18n.uploadmanual,
			position: "right",
			callback: () => this.uploadAll(this.currentPageId)
		});

		this.eventBus.on("open-menu-image", this.injectContextMenu);
		this.eventBus.on("open-menu-link", this.injectContextMenu); 
		
		//switch-protyle 是思源（SiYuan）里“编辑器上下文切换”时触发的事件，通常发生在：
		//切换到另一个文档
		//同一文档但焦点切到另一个 protyle 实例（如分屏/页签切换）
		//编辑区域激活对象变化
		this.eventBus.on("switch-protyle", async (data) => {
			this.currentPageId = data.detail.protyle.block.rootID;

		});
		
		//click-editorcontent 是思源里“点击编辑器内容区域”时触发的事件。
		this.eventBus.on("click-editorcontent", async (data) => {
			this.currentPageId = data.detail.protyle.block.rootID;
		});


		this.eventBus.on("open-menu-doctree", this.doctreeMenuEventListener);

		this.registerSettingUI();
		// 在插件卸载时清理事件监听器
		const originalOnunload = this.onunload;
		this.onunload = async () => {
			this.eventBus.off("open-menu-doctree", this.doctreeMenuEventListener);
			if (originalOnunload) {
				await originalOnunload.call(this);
			}
		};
	}
	/* 文档树菜单弹出事件监听器 */
	protected readonly doctreeMenuEventListener = (e: any) => {
		// this.logger.debug(e);
		const submenu: any[] = [];
		switch (e.detail.type) {
		case "doc": {
			// 单文档
			const id = e.detail.elements.item(0)?.dataset?.nodeId;

			if (id) {
			submenu.push(
				{
				icon: "iconFile",
				label: "导出md文件",
				click: async () => {
					try {
						// 获取文档信息 md 内容
					const res = await this._exportMdContent(id);
					if (res?.code !== 0 || !res?.data) {
						showMessage("导出失败：内核未返回内容", 5000, "error");
						return;
					}
					const processedContent = await this.processMarkdownContent(res.data, id);
					if (processedContent == null || processedContent.content === "") {
						showMessage("导出失败：没有可写入的正文", 5000, "error");
						return;
					}
					const blob = new Blob([processedContent.content], {
						type: "text/markdown;charset=utf-8",
					});
					const url = URL.createObjectURL(blob);
					const a = document.createElement("a");
					a.href = url;
					a.download = `${processedContent.title}.md`;
					document.body.appendChild(a);
					a.click();
					setTimeout(() => {
						document.body.removeChild(a);
						URL.revokeObjectURL(url);
					}, 100);
					showMessage("文件已下载", 3000, "info");
					} catch (e) {
					console.error(e);
					showMessage("导出异常，请查看控制台", 5000, "error");
					}
				},
				},
				{
				icon: "iconUpload",
				label: "仅上传图床",
				click: async () => {
					// 上传图床
					this.currentPageId = id;
					await this.uploadAll(id);
				},
				},
				
			);
			}
			break;
		}
		case "docs": {
			// 多文档
			const ids: string[] = [];
			// 遍历所有选中的元素
			for (let i = 0; i < e.detail.elements.length; i++) {
			const element = e.detail.elements.item(i);
			if (element && element.dataset.nodeId) {
				ids.push(element.dataset.nodeId);
			}
			}

			if (ids.length > 0) {
			submenu.push({
				icon: "iconFile",
				label: "批量导出为ZIP",
				click: async () => {
				await this.batchExportMdToZip(ids);
				},
			});
			}
			break;
		}
		default:
			break;
		}

		if (submenu.length > 0) {
		e.detail.menu.addItem({
			icon: "iconCode",
			label: this.displayName,
			submenu,
		});
		}
	}
	private newServerId(): string {
		return `srv-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
	}

	/** 从旧版单 baseURL 迁移为多服务器列表 */
	private migrateServerConfig(): void {
		const c = this.config;
		const defaultName = (this.i18n as any)?.defaultServerName ?? "默认";
		if (!Array.isArray(c.servers) || c.servers.length === 0) {
			const id = this.newServerId();
			c.servers = [
				{
					id,
					name: defaultName,
					baseURL: String(c.baseURL ?? "").trim(),
					authHeader: String(c.authHeader ?? "").trim(),
					enabled: true,
				},
			];
			c.currentServerId = id;
		} else {
			for (const s of c.servers as ServerEntry[]) {
				if (!s.id) {
					s.id = this.newServerId();
				}
				if (!s.name) {
					s.name = defaultName;
				}
				s.baseURL = String(s.baseURL ?? "").trim();
				s.authHeader = String(s.authHeader ?? "").trim();
				if (typeof s.enabled !== "boolean") {
					s.enabled = true;
				}
			}
			if (!c.currentServerId || !c.servers.some((x: ServerEntry) => x.id === c.currentServerId)) {
				c.currentServerId = (c.servers[0] as ServerEntry).id;
			}
		}
		if (typeof c.syncToAllServers !== "boolean") {
			c.syncToAllServers = false;
		}
	}

	private getServers(): ServerEntry[] {
		return Array.isArray(this.config.servers) ? this.config.servers : [];
	}

	private getCurrentServer(): ServerEntry | null {
		const id = this.config.currentServerId as string | undefined;
		const list = this.getServers();
		if (!id) {
			return list[0] ?? null;
		}
		return list.find((s) => s.id === id) ?? list[0] ?? null;
	}

	private isServerUploadEnabled(s: ServerEntry): boolean {
		return s.enabled !== false;
	}

	/** 勾选「同步到全部服务器」时返回所有已启用且已填地址的服务器；否则仅当前选中（须启用） */
	private getUploadTargets(): ServerEntry[] {
		const list = this.getServers().filter((s) => this.isServerUploadEnabled(s) && this.normBaseURL(s));
		if (list.length === 0) {
			return [];
		}
		if (this.config.syncToAllServers) {
			return list;
		}
		const cur = this.getCurrentServer();
		return cur && this.isServerUploadEnabled(cur) && this.normBaseURL(cur) ? [cur] : [];
	}

	private noUploadTargetHint(): string {
		const all = this.getServers();
		const hasUrl = all.some((s) => this.normBaseURL(s));
		const hasEnabledWithUrl = all.some((s) => this.isServerUploadEnabled(s) && this.normBaseURL(s));
		if (all.length > 0 && hasUrl && !hasEnabledWithUrl) {
			return (
				(this.i18n as any).allServersDisabled ??
				"没有已启用的服务器：请在设置中为至少一台服务器勾选「启用」。"
			);
		}
		return (
			(this.i18n as any).noServerConfigured ?? "请先在设置中填写至少一个服务器地址"
		);
	}

	onunload() {
		this.eventBus.off("open-menu-image", this.injectContextMenu);
		this.eventBus.off("open-menu-link", this.injectContextMenu); 
	}

	private registerSettingUI() {
		this.setting = new Setting({
			confirmCallback: () => this.saveData(STORAGE_KEY, this.config),
		});

		const serverSelect = document.createElement("select");
		serverSelect.className = "b3-select fn__block custompic-setting-input";

		const nameInput = document.createElement("input");
		nameInput.className = "b3-text-field fn__block custompic-setting-input";
		nameInput.placeholder = (this.i18n as any).serverNamePlaceholder ?? "备注名";
		const baseInput = document.createElement("input");
		baseInput.className = "b3-text-field fn__block custompic-setting-input";
		baseInput.placeholder = "http://your-server-url";
		const authInput = document.createElement("input");
		authInput.className = "b3-text-field fn__block custompic-setting-input";
		authInput.placeholder = "xxxxxx...API Key";

		const syncAll = document.createElement("input");
		syncAll.type = "checkbox";
		syncAll.id = "custompic-sync-all";
		const syncLabel = document.createElement("label");
		syncLabel.htmlFor = "custompic-sync-all";
		syncLabel.textContent = (this.i18n as any).syncToAllServers ?? "同步上传到全部已配置服务器";
		const syncWrap = document.createElement("div");
		syncWrap.className = "fn__flex";
		syncWrap.append(syncAll, syncLabel);

		const enabledInput = document.createElement("input");
		enabledInput.type = "checkbox";
		enabledInput.id = "custompic-server-enabled";
		const enabledLabel = document.createElement("label");
		enabledLabel.htmlFor = "custompic-server-enabled";
		enabledLabel.textContent =
			(this.i18n as any).serverEnabled ?? "启用此服务器（参与上传）";
		const enabledWrap = document.createElement("div");
		enabledWrap.className = "fn__flex";
		enabledWrap.append(enabledInput, enabledLabel);

		const formatOptionLabel = (s: ServerEntry): string => {
			const base = s.name || s.baseURL || s.id;
			const mark = (this.i18n as any).serverDisabledMark ?? "已禁用";
			return s.enabled === false ? `${base}（${mark}）` : base;
		};

		const currentServer = (): ServerEntry | null => {
			const id = this.config.currentServerId;
			return this.getServers().find((s) => s.id === id) ?? null;
		};

		const fillInputs = () => {
			const s = currentServer();
			if (!s) {
				return;
			}
			nameInput.value = s.name || "";
			baseInput.value = s.baseURL || "";
			authInput.value = s.authHeader || "";
			enabledInput.checked = s.enabled !== false;
		};

		const fillSelect = () => {
			serverSelect.innerHTML = "";
			for (const s of this.getServers()) {
				const opt = document.createElement("option");
				opt.value = s.id;
				opt.textContent = formatOptionLabel(s);
				serverSelect.appendChild(opt);
			}
			serverSelect.value = this.config.currentServerId || "";
			if (!serverSelect.value && this.getServers()[0]) {
				this.config.currentServerId = this.getServers()[0].id;
				serverSelect.value = this.config.currentServerId;
			}
			fillInputs();
		};

		fillSelect();

		serverSelect.addEventListener("change", () => {
			this.config.currentServerId = serverSelect.value;
			fillInputs();
		});

		enabledInput.addEventListener("change", () => {
			const s = currentServer();
			if (s) {
				s.enabled = enabledInput.checked;
				const opt = Array.from(serverSelect.options).find((o) => o.value === s.id);
				if (opt) {
					opt.textContent = formatOptionLabel(s);
				}
			}
		});

		nameInput.addEventListener("input", () => {
			const s = currentServer();
			if (s) {
				s.name = nameInput.value;
				const opt = Array.from(serverSelect.options).find((o) => o.value === s.id);
				if (opt) {
					opt.textContent = formatOptionLabel(s);
				}
			}
		});
		baseInput.addEventListener("input", () => {
			const s = currentServer();
			if (s) {
				s.baseURL = baseInput.value;
				const opt = Array.from(serverSelect.options).find((o) => o.value === s.id);
				if (opt) {
					opt.textContent = formatOptionLabel(s);
				}
			}
		});
		authInput.addEventListener("input", () => {
			const s = currentServer();
			if (s) {
				s.authHeader = authInput.value;
			}
		});

		syncAll.checked = !!this.config.syncToAllServers;
		syncAll.addEventListener("change", () => {
			this.config.syncToAllServers = syncAll.checked;
		});

		const addBtn = document.createElement("button");
		addBtn.className = "b3-button";
		addBtn.textContent = (this.i18n as any).addServer ?? "添加服务器";
		addBtn.onclick = () => {
			const s: ServerEntry = {
				id: this.newServerId(),
				name: `${(this.i18n as any).serverPrefix ?? "服务器"} ${this.getServers().length + 1}`,
				baseURL: "",
				authHeader: "",
				enabled: true,
			};
			this.getServers().push(s);
			this.config.currentServerId = s.id;
			fillSelect();
		};

		const delBtn = document.createElement("button");
		delBtn.className = "b3-button";
		delBtn.textContent = (this.i18n as any).removeServer ?? "删除当前服务器";
		delBtn.onclick = () => {
			if (this.getServers().length <= 1) {
				showMessage((this.i18n as any).needOneServer ?? "至少保留一个服务器配置", 4000, "info");
				return;
			}
			const id = this.config.currentServerId;
			this.config.servers = this.getServers().filter((x) => x.id !== id);
			this.config.currentServerId = this.getServers()[0]?.id ?? "";
			fillSelect();
		};

		const btnRow = document.createElement("div");
		btnRow.className = "fn__flex";
		btnRow.style.gap = "8px";
		btnRow.append(addBtn, delBtn);

		const testBtn = document.createElement("button");
		testBtn.className = "b3-button";
		testBtn.textContent = this.i18n.testconnect;
		testBtn.onclick = async () => {
			const r = await this.testConnection();
			if (r.ok) {
				showMessage(this.i18n.connectSuccess, 5000, "info");
			} else {
				showMessage(
					r.timedOut ? this.i18n.connectTimeout : this.i18n.connectFail,
					r.timedOut ? 8000 : 5000,
					"error",
				);
			}
		};

		this.setting.addItem({
			title: (this.i18n as any).selectServer ?? "当前服务器",
			createActionElement: () => serverSelect,
		});
		this.setting.addItem({ title: (this.i18n as any).serverDisplayName ?? "显示名称", createActionElement: () => nameInput });
		this.setting.addItem({
			title: (this.i18n as any).serverEnabledTitle ?? "启用",
			createActionElement: () => enabledWrap,
		});
		this.setting.addItem({ title: this.i18n.paperlessAddr, createActionElement: () => baseInput });
		this.setting.addItem({ title: this.i18n.paperlessToken, createActionElement: () => authInput });
		this.setting.addItem({ title: (this.i18n as any).multiServerActions ?? "列表", createActionElement: () => btnRow });
		this.setting.addItem({ title: (this.i18n as any).syncToAllServers ?? "同步到全部", createActionElement: () => syncWrap });
		this.setting.addItem({ title: this.i18n.testconnect, actionElement: testBtn });

		this.supportedSuffixes = this.parseSuffixes(DEFAULT_FILE_SUFFIXES);
	}

	private parseSuffixes(raw: string): string[] {
		return raw.split(/[\,\s]+/).map(s => s.replace("*", "").trim()).filter(Boolean);
	}

	private getAuthHeader(server?: ServerEntry | null): string {
		const s = server ?? this.getCurrentServer();
		const raw = (s?.authHeader ?? "").trim();
		if (!raw) {
			return "";
		}
		return raw.startsWith("Token ") ? raw : `Token ${raw}`;
	}
	
	private async _exportMdContent(
		id: string,
	  ): Promise<any> {
		return await fetchSyncPost("/api/export/exportMdContent", {
			id: id,
		}) as any;
	  }
	  
	/**
	 * 去掉导出内容开头的 YAML front matter，再前置 buildDocFrontMatter 生成的新 front matter。
	 * 原先用 .then 拼接但未 await，return 时异步尚未完成，导致下载文件里没有新 front matter。
	 */
	private async processMarkdownContent(data: any, doc_id: string): Promise<{content: string, title: string} | null> {
		const raw = typeof data?.content === "string" ? data.content : "";
		if (!raw) {
			return null;
		}
		let body = raw.replace(/^---\s*\r?\n[\s\S]*?\r?\n---\s*\r?\n?/, "");
		const front = await this.buildDocFrontMatter(doc_id);
		if (!front.front_matter) {
			return {content: body, title: front.title};
		}
		const sep = body.startsWith("\n") || body === "" ? "" : "\n";
		return {content: `${front.front_matter}${sep}${body}`, title: front.title};
	}
	/** 格式化为 `2021-09-13 14:29:53`（本地时区） */
	private formatLocalDateTime(dt: Date): string {
		const p = (n: number) => String(n).padStart(2, "0");
		return `${dt.getFullYear()}-${p(dt.getMonth() + 1)}-${p(dt.getDate())} ${p(dt.getHours())}:${p(dt.getMinutes())}:${p(dt.getSeconds())}`;
	}

	/** 思源 ial.updated 常见为 14 位 yyyyMMddHHmmss，不能交给 Date 直接解析 */
	private siyuanUpdatedToIso(updated: string | undefined): string {
		const s = (updated ?? "").trim();
		if (/^\d{14}$/.test(s)) {
			const y = +s.slice(0, 4);
			const mo = +s.slice(4, 6) - 1;
			const d = +s.slice(6, 8);
			const h = +s.slice(8, 10);
			const mi = +s.slice(10, 12);
			const se = +s.slice(12, 14);
			const dt = new Date(y, mo, d, h, mi, se);
			return Number.isNaN(dt.getTime()) ? this.formatLocalDateTime(new Date()) : this.formatLocalDateTime(dt);
		}
		const t = Date.parse(s);
		if (!Number.isNaN(t)) {
			return this.formatLocalDateTime(new Date(t));
		}
		return this.formatLocalDateTime(new Date());
	}

	/** 生成合法 YAML 标量（必要时用 JSON 双引号转义） */
	private yamlQuoteScalar(v: string): string {
		if (!v) {
			return '""';
		}
		if (/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(v)) {
			return JSON.stringify(v);
		}
		if (/[\n"#]/.test(v) || v !== v.trim() || /: /.test(v)) {
			return JSON.stringify(v);
		}
		return v;
	}

	private yamlStringList(key: string, items: string[]): string {
		if (!items.length) {
			return `${key}: []\n`;
		}
		return `${key}:\n${items.map((x) => `  - ${this.yamlQuoteScalar(x)}`).join("\n")}\n`;
	}

	/**
	 * 根据 getDocInfo + 文档路径生成 VuePress 风格 front matter。
	 * 失败时返回空字符串（调用方需判断）。
	 */
	private async buildDocFrontMatter(doc_id: string): Promise<{front_matter: string, title: string}> {
		const docInfo = await fetchSyncPost("/api/block/getDocInfo", { id:doc_id}) as any;
		if (docInfo?.code !== 0 || !docInfo?.data) {
			return {front_matter: "", title: ""};
		}
		const data = docInfo.data;
		const ial = data.ial ?? {};
		const title = String(data.name ?? ial.title ?? "").trim();
		const datetimeStr = this.siyuanUpdatedToIso(ial.updated);
		const permalink = data.rootID || doc_id;

		let fullHPath = "";
		let pathRes = (await fetchSyncPost("/api/filetree/getFullHPathByID", { id:doc_id })) as any;
		if (pathRes?.code !== 0) {
			pathRes = (await fetchSyncPost("/api/filetree/getHPathByID", { id:doc_id })) as any;
		}
		if (pathRes?.code === 0 && pathRes.data != null) {
			fullHPath = typeof pathRes.data === "string" ? pathRes.data : "";
		}

		const segments = fullHPath.split("/").map((p) => p.trim()).filter(Boolean);
		const categorySegments = segments.slice(0, -1);

		const tagsRaw = ial.tags;
		const tags =
			typeof tagsRaw === "string"
				? tagsRaw.split(/[\s,，、]+/).map((t) => t.trim()).filter(Boolean)
				: [];

		const categoriesYaml = this.yamlStringList("categories", categorySegments);
		const tagsYaml = this.yamlStringList("tags", tags);

		const front_matter = `---
title: ${this.yamlQuoteScalar(title)}
date: ${this.yamlQuoteScalar(datetimeStr)}
permalink: /pages/${permalink}
${categoriesYaml}${tagsYaml}---
`;
		return {front_matter: front_matter, title: title};
	}

	private safeMdFileBaseName(title: string, id: string): string {
		const raw = title && title.trim() ? title.trim() : id;
		const cleaned = raw.replace(/[<>:"/\\|?*\x00-\x1f]/g, "_").replace(/\s+/g, " ").trim();
		return cleaned || id;
	}

	private pickUniqueMdFileName(base: string, used: Set<string>): string {
		let stem = base.slice(0, 180);
		if (!stem) {
			stem = "untitled";
		}
		let name = `${stem}.md`;
		let n = 2;
		while (used.has(name)) {
			name = `${stem}_${n}.md`;
			n++;
		}
		used.add(name);
		return name;
	}

	private async batchExportMdToZip(docIds: string[]): Promise<void> {
		if (!docIds.length) {
			return;
		}
		const zip = new JSZip();
		const usedNames = new Set<string>();
		let ok = 0;
		try {
			showMessage(`正在导出 0/${docIds.length} …`, -1, "info", "custompic-batch-zip");
			for (let i = 0; i < docIds.length; i++) {
				const docId = docIds[i];
				const res = await this._exportMdContent(docId);
				if (res?.code !== 0 || !res?.data) {
					continue;
				}
				const proc = await this.processMarkdownContent(res.data, docId);
				if (!proc?.content) {
					continue;
				}
				const base = this.safeMdFileBaseName(proc.title, docId);
				const fileName = this.pickUniqueMdFileName(base, usedNames);
				zip.file(fileName, proc.content);
				ok++;
				showMessage(`正在导出 ${ok}/${docIds.length} …`, -1, "info", "custompic-batch-zip");
			}
			if (ok === 0) {
				showMessage(
					"批量导出失败：未能生成任何 .md（请检查内核导出与文档权限）",
					6000,
					"error",
					"custompic-batch-zip",
				);
				return;
			}
			const blob = await zip.generateAsync({
				type: "blob",
				compression: "DEFLATE",
				compressionOptions: { level: 6 },
			});
			const url = URL.createObjectURL(blob);
			const a = document.createElement("a");
			a.href = url;
			a.download = `export-batch-${ok}-of-${docIds.length}-${new Date()
				.toISOString()
				.slice(0, 19)
				.replace(/:/g, "-")}.zip`;
			document.body.appendChild(a);
			a.click();
			setTimeout(() => {
				document.body.removeChild(a);
				URL.revokeObjectURL(url);
			}, 200);
			showMessage(`已打包下载 ${ok}/${docIds.length} 个文档`, 4000, "info", "custompic-batch-zip");
		} catch (e) {
			console.error(e);
			showMessage("批量导出异常，请查看控制台", 6000, "error", "custompic-batch-zip");
		}
	}

	/** 与 isPrivateBaseURL 一致，仅判断 hostname（用于超时时间等） */
	private isPrivateHostname(hostname: string): boolean {
		const h = (hostname || "").toLowerCase();
		if (h === "localhost" || h === "127.0.0.1" || h === "::1" || h === "[::1]") {
			return true;
		}
		const ip = h.replace(/^\[|\]$/g, "");
		const m = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.exec(ip);
		if (!m) {
			return false;
		}
		const a = +m[1], b = +m[2], c = +m[3], d = +m[4];
		if (a === 10) return true;
		if (a === 172 && b >= 16 && b <= 31) return true;
		if (a === 192 && b === 168) return true;
		if (a === 127) return true;
		if (a === 169 && b === 254) return true;
		if (a === 0) return true;
		return false;
	}

	/** 内网/本机：须直连 fetch；思源 forwardProxy 会禁止访问这些地址 */
	private isPrivateBaseURL(base: string): boolean {
		try {
			return this.isPrivateHostname(new URL(base).hostname);
		} catch {
			return true;
		}
	}

	/** 局域网直连可能较慢，适当延长；公网略放宽默认 8s */
	private connectionTestTimeoutMs(fullUrl: string): number {
		try {
			return this.isPrivateHostname(new URL(fullUrl).hostname) ? 15000 : 8000;
		} catch {
			return 8000;
		}
	}

	private async testConnectionWithFetch(fullUrl: string): Promise<"ok" | "fail" | "timeout"> {
		const ms = this.connectionTestTimeoutMs(fullUrl);
		const ctrl = new AbortController();
		const timer = window.setTimeout(() => ctrl.abort(), ms);
		try {
			const headers: Record<string, string> = {};
			const auth = this.getAuthHeader();
			if (auth) {
				headers["Authorization"] = auth;
			}
			const res = await fetch(fullUrl, { method: "GET", headers, signal: ctrl.signal });
			window.clearTimeout(timer);
			if (!res.ok) {
				return "fail";
			}
			const text = await res.text();
			if (!text) {
				return "fail";
			}
			try {
				const json = JSON.parse(text);
				return json?.success === true ? "ok" : "fail";
			} catch {
				return "fail";
			}
		} catch (e: unknown) {
			window.clearTimeout(timer);
			const name =
				e && typeof e === "object" && "name" in e ? String((e as { name: string }).name) : "";
			const msg =
				e && typeof e === "object" && "message" in e
					? String((e as { message: string }).message)
					: "";
			// AbortController 超时；部分环境下 TCP 超时表现为 Failed to fetch / net::ERR_CONNECTION_TIMED_OUT
			if (name === "AbortError" || /timeout|timed out|TIMED_OUT|ETIMEDOUT|ERR_CONNECTION_TIMED_OUT/i.test(msg)) {
				return "timeout";
			}
			return "fail";
		}
	}

	/** 公网地址在浏览器端可能因 CORS / 混合内容导致 fetch 失败，由内核代发可绕过 */
	private testConnectionWithForwardProxy(fullUrl: string): Promise<boolean> {
		return new Promise((resolve) => {
			fetchPost(
				"/api/network/forwardProxy",
				{
					url: fullUrl,
					method: "GET",
					headers: [{ Authorization: this.getAuthHeader() }],
					responseEncoding: "text",
					timeout: 8000,
				},
				(res: any) => {
					const status = res?.data?.status;
					const body = res?.data?.body;
					if (status === 200 && typeof body === "string") {
						try {
							const json = JSON.parse(body);
							resolve(json?.success === true);
							return;
						} catch {
							/* ignore */
						}
					}
					resolve(false);
				}
			);
		});
	}

	/** 内网地址只能直连；超时多为服务未起、防火墙或未路由到该 IP（控制台 net::ERR_CONNECTION_TIMED_OUT） */
	private async testConnection(): Promise<{ ok: boolean; timedOut?: boolean }> {
		const base = this.normBaseURL();
		if (!base) {
			return { ok: false };
		}
		const url = `${base}/api/testConnection`;

		// 优先直连，避免内核 forwardProxy 对内网地址的限制。
		const direct = await this.testConnectionWithFetch(url);
		if (direct === "ok") {
			return { ok: true };
		}
		if (direct === "timeout") {
			return { ok: false, timedOut: true };
		}

		// 内网/本机地址不再走 forwardProxy，避免触发 ip prohibited。
		if (this.isPrivateBaseURL(base)) {
			return { ok: false };
		}

		// 公网地址在浏览器端可能被 CORS 拦截，回退到内核代发。
		const viaProxy = await this.testConnectionWithForwardProxy(url);
		return { ok: viaProxy };
	}
	/**
	 * 上传当前编辑文档中的所有资源
	 */
	private async uploadAll(doc_id: string) {
		//try {
			const recentblock: any = await fetchSyncPost("/api/block/getChildBlocks", {
				id: doc_id
			});
			console.log("recentblock", doc_id);
			if (recentblock?.code !== 0) {
				showMessage(this.i18n.failgetassets, 5000, "error");
				return;
			}

			const blocks: any[] = recentblock?.data || [];
			const refs = this.collectBlockAssetRefs(blocks);
			const paths = Array.from(new Set(refs.map((r) => r.path)));

			if (paths.length === 0) {
				showMessage(this.i18n.noinvaildassets, 5000, "info");
				return;
			}
			console.log("paths", paths);

			let successCount = 0;
			let skipCount = 0;
			for (const path of paths) {
				const uploadPath = this.toUploadAssetPath(path);
				if (!uploadPath) continue;
				const name = uploadPath.split("/").pop() || "";
				if (!name) continue;
				// if (await this.documentExists(uploadPath)) {
				//     skipCount++;
				//     continue;
				// }
				// 不在此传入 currentPageId：那是文档根块，不是资源所在子块；替换用下方 blockAssetMap 中的真实 blockId。
				const ok = await this.uploadFile(uploadPath, name);
				if (ok) {
					successCount++;
					const publicUrl = this.imageMap.get(uploadPath) || null;
					if (publicUrl) {
						const blockIds = this.blockAssetMap.get(path);
						if (blockIds && blockIds.size > 0) {
							for (const blockId of blockIds) {
								await this.replaceBlockAssetWithUrlById(blockId, path, name, publicUrl, false);
							}
						}
					}
				}
			}

			const msg = this.i18n.uploadSummary
				.replace("${total}", paths.length.toString())
				.replace("${success}", successCount.toString())
				.replace("${skipped}", skipCount.toString());
			showMessage(msg, 6000, "info");
		// } catch (e) {
		//     console.error("uploadAll", e);
		//     showMessage(this.i18n.failgetassets, 5000, "error");
		// }
	}

	private extractAssetPaths(text: string): string[] {
		// 仅提取链接目标（() / src / href），避免误命中 [] 中的 alt text。
		const mdLinkTargetRegex = /!?\[[^\]]*]\(([^)]+)\)/g;
		const htmlAttrRegex = /\b(?:src|href)\s*=\s*["']([^"']+)["']/gi;
		const out: string[] = [];
		const seen = new Set<string>();
		const normalize = (raw: string): string | null => {
			let url = (raw || "").trim();
			if (!url) return null;
			// markdown 支持 (<url>) 与 (url "title") 形式
			if (url.startsWith("<") && url.endsWith(">")) {
				url = url.slice(1, -1).trim();
			} else {
				url = url.split(/\s+/)[0];
			}
			if (!url) return null;
			// 保留完整 URL（含 query/hash），不去参数、不拼接，后续上传前再标准化。
			if (
				url.includes("/data/assets/") ||
				url.includes("/assets/") ||
				url.startsWith("assets/")
			) {
				return url;
			}
			return null;
		};

		let m: RegExpExecArray | null = null;
		while ((m = mdLinkTargetRegex.exec(text)) !== null) {
			const normalized = normalize(m[1] || "");
			if (!normalized) continue;
			if (!seen.has(normalized)) {
				seen.add(normalized);
				out.push(normalized);
			}
		}

		while ((m = htmlAttrRegex.exec(text)) !== null) {
			const normalized = normalize(m[1] || "");
			if (!normalized) continue;
			if (!seen.has(normalized)) {
				seen.add(normalized);
				out.push(normalized);
			}
		}

		return out;
	}

	/** 将提取到的资源 URL 转为 /api/file/getFile 可读路径（上传前使用）。 */
	private toUploadAssetPath(rawPath: string): string | null {
		let p = (rawPath || "").trim();
		if (!p) return null;
		if (p.startsWith("http://") || p.startsWith("https://")) return null;
		p = p.split("?")[0].split("#")[0];

		const fromData = p.match(/\/data\/assets\/([^?#]+)/);
		if (fromData) {
			return `/data/assets/${fromData[1].replace(/^\/+/, "")}`;
		}
		const fromAssetsAbs = p.match(/\/assets\/([^?#]+)/);
		if (fromAssetsAbs) {
			return `/data/assets/${fromAssetsAbs[1].replace(/^\/+/, "")}`;
		}
		if (p.startsWith("assets/")) {
			return `/data/assets/${p.slice("assets/".length).replace(/^\/+/, "")}`;
		}
		return null;
	}
	/**
	 * 收集 block 中出现的资源路径与 blockId，结果同时缓存到 blockAssetMap。
	 */
	private collectBlockAssetRefs(blocks: any[]): Array<{ blockId: string; path: string; name: string }> {
		this.blockAssetMap.clear();
		const refs: Array<{ blockId: string; path: string; name: string }> = [];

		for (const block of blocks || []) {
			const blockId = block?.id as string | undefined;
			const markdown = (block?.markdown || "") as string;
			if (!blockId || !markdown) continue;

			for (const path of this.extractAssetPaths(markdown)) {
				const uploadPath = this.toUploadAssetPath(path);
				if (!uploadPath) continue;
				const name = uploadPath.split("/").pop() || "";
				if (!name) continue;
				if (!this.supportedSuffixes.some((suffix) => name.endsWith(suffix))) continue;

				refs.push({ blockId, path, name });
				if (!this.blockAssetMap.has(path)) {
					this.blockAssetMap.set(path, new Set<string>());
				}
				this.blockAssetMap.get(path)!.add(blockId);
			}
		}

		return refs;
	}

	private injectContextMenu = ({ detail }: any) => {
		const el = detail.element;
		let filePath = "";

		const imgEl = el.querySelector("img");
		const src = imgEl?.getAttribute("src") || el?.dataset?.href || "";

		if (src.includes("assets/")) {
			filePath = src;
		} else if (src.includes("/assets/")) {
			const match = src.match(/\/assets\/([^?#]+)/);
			if (match) filePath = `assets/${match[1]}`;
		}

		const name = filePath.split("/").pop();
		if (!filePath || !name || !this.supportedSuffixes.some(s => name.endsWith(s))) return;
		//如果不是本地资源，就退出，提示信息，资源为网络资源，请下载之后再上传
		if (filePath.startsWith("http://") || filePath.startsWith("https://")) {
			//console.log("网络资源，请下载到本地之后再上传", filePath);
			//showMessage(this.i18n.networkResource, 5000, "info");
			return;
		}

		detail.menu.addItem({
			icon: "iconUpload",
			label: this.i18n.uploadmanual,
			click: async () => {
				// 点击上传时再校验连通性，避免每次打开右键菜单都发请求。
				const conn = await this.testConnection();
				if (!conn.ok) {
					showMessage(
						conn.timedOut ? this.i18n.connectTimeout : this.i18n.connectFail,
						conn.timedOut ? 8000 : 5000,
						"error",
					);
					return;
				}
				// if (await this.documentExists(`/data/${filePath}`)) {
				//     showMessage(this.i18n.docExistsSkip.replace("${name}", name));
				//     return;
				// }
				await this.uploadFile(`/data/${filePath}`, name, {
					menuElement: el,
					assetSrc: src,
				});
			}
		});
	};      

	private async documentExists(path: string): Promise<boolean> {
		const searchUrl = `${this.normBaseURL()}/api/documentExists/?path=${encodeURIComponent(path)}`;
		try {
			const res = await fetch(searchUrl, {
				method: "GET",
				headers: { Authorization: this.getAuthHeader() }
			});
			if (!res.ok) {
				return false;
			}
			const text = await res.text();
			if (!text) {
				return false;
			}
			let json: any = null;
			try {
				json = JSON.parse(text);
			} catch {
				return false;
			}
			return Array.isArray(json?.results) && json.results.some((doc: any) => doc.path === path);
		} catch (e) {
			console.warn(this.i18n.failedsearch, e);
			return false;
		}
	}

	private normBaseURL(server?: ServerEntry | null): string {
		const s = server ?? this.getCurrentServer();
		return ((s?.baseURL ?? "") as string).replace(/\/+$/, "");
	}

	/** 解析上传接口响应：支持 { success, public_url }、{ file_url, id }、task_id、纯 UUID 文本等 */
	private parseUploadResult(
		rawBody: string,
		responseOk: boolean,
		baseUrl: string,
	): { success: boolean; publicUrl: string | null; message: string | null } {
		if (!responseOk) {
			return { success: false, publicUrl: null, message: "unknown error" };
		}
		const trimmed = rawBody.trim();
		const base = baseUrl;
		const toAbsolute = (p: string): string => {
			const t = p.trim();
			return t.startsWith("http://") || t.startsWith("https://")
				? t
				: `${base}${t.startsWith("/") ? "" : "/"}${t}`;
		};

		try {
			const j = JSON.parse(trimmed);
			if (typeof j?.success === "boolean") {
				if (!j.success) {
					return { success: false, publicUrl: null ,message:j.message};
				}
				const fromPublic = j.public_url ?? j.publicUrl;
				if (typeof fromPublic === "string" && fromPublic.trim()) {
					return { success: true, publicUrl: toAbsolute(fromPublic) ,message:j.message};
				}
				if (j?.task_id) {
					return { success: true, publicUrl: null ,message:j.message};
				}
				if (j?.file_url && typeof j.file_url === "string") {
					return { success: true, publicUrl: toAbsolute(j.file_url) ,message:j.message};
				}
				if (j?.id && typeof j.id === "string" && /^[a-f0-9-]{36}$/i.test(j.id)) {
					return { success: true, publicUrl: `${base}/api/files/${j.id}` ,message:j.message};
				}
				return { success: true, publicUrl: null ,message:j.message};
			}
			if (j?.task_id) {
				return { success: true, publicUrl: null ,message:j.message};
			}
			if (j?.file_url && typeof j.file_url === "string") {
				return { success: true, publicUrl: toAbsolute(j.file_url) ,message:j.message};
			}
			if (j?.id && typeof j.id === "string" && /^[a-f0-9-]{36}$/i.test(j.id)) {
				return { success: true, publicUrl: `${base}/api/files/${j.id}` ,message:j.message};
			}
		} catch {
			/* 非 JSON */
		}

		const uuidPlain = trimmed.replace(/^"|"$/g, "");
		if (/^[a-f0-9-]{36}$/i.test(uuidPlain)) {
			return { success: true, publicUrl: `${base}/api/files/${uuidPlain}`, message: null };
		}
		return { success: false, publicUrl: null ,message:"unknown error"};
	}

	private assetPathCandidates(assetSrc: string, name: string): string[] {
		const out: string[] = [];
		const seen = new Set<string>();
		const add = (s: string) => {
			const t = s.trim();
			if (t && !seen.has(t)) {
				seen.add(t);
				out.push(t);
			}
		};
		add(assetSrc);
		add(`assets/${name}`);
		const m1 = assetSrc.match(/\/assets\/([^?#]+)/);
		if (m1) {
			add(`assets/${m1[1]}`);
		}
		try {
			const u = new URL(assetSrc, "http://127.0.0.1");
			const m2 = u.pathname.match(/\/assets\/(.+)$/);
			if (m2) {
				add(`assets/${m2[1]}`);
			}
		} catch {
			/* ignore */
		}
		return out;
	}

	private truncateForMessage(text: string, maxLen = 200): string {
		if (text.length <= maxLen) {
			return text;
		}
		return `${text.slice(0, maxLen - 1)}…`;
	}

	/**
	 * 按 blockId 将块中的资源路径替换为公网地址（供 uploadAll 批量处理）。
	 */
	private async replaceBlockAssetWithUrlById(
		blockId: string,
		assetSrc: string,
		name: string,
		publicUrl: string,
		notify = false,
	): Promise<boolean> {
		try {
			const res = await fetchSyncPost("/api/block/getBlockKramdown", { id: blockId }) as any;
			if (res.code !== 0) {
				throw new Error(res.msg || "");
			}
			let kd = res.data?.kramdown as string;
			if (!kd) {
				throw new Error("no kramdown");
			}
			const candidates = this.assetPathCandidates(assetSrc, name);
			let replaced = false;
			for (const c of candidates) {
				if (c && kd.includes(c)) {
					kd = kd.split(c).join(publicUrl);
					replaced = true;
					break;
				}
			}
			if (!replaced) {
				if (notify) {
					showMessage(this.i18n.linkReplaceNoMatch, 6000, "info");
				}
				return false;
			}
			const ures = await fetchSyncPost("/api/block/updateBlock", {
				id: blockId,
				dataType: "markdown",
				data: kd,
			}) as any;
			if (ures.code !== 0) {
				throw new Error(ures.msg || "");
			}
			if (notify) {
				const urlLine = this.truncateForMessage(publicUrl);
				showMessage(this.i18n.linkReplacedDone.replace("${url}", urlLine), 2000, "info");
			}
			return true;
		} catch (e) {
			console.error("replaceBlockAssetWithUrlById", e);
			if (notify) {
				showMessage(this.i18n.linkReplaceFail, 5000, "error");
			}
			return false;
		}
	}

	/**
	 * 仅处理 menuElement 类型：从右键触发元素定位 blockId，再调用 block 替换方法。
	 */
	private async replaceEditorAssetWithUrl(
		menuElement: HTMLElement,
		assetSrc: string,
		name: string,
		publicUrl: string,
	): Promise<boolean> {
		const blockEl = menuElement.closest("[data-node-id]");
		const blockId = blockEl?.getAttribute("data-node-id");
		if (!blockId) {
			showMessage(this.i18n.linkReplaceNoBlock, 5000, "info");
			return false;
		}
		return await this.replaceBlockAssetWithUrlById(blockId, assetSrc, name, publicUrl, true);
	}

	/**
	 * 用 XHR 读内核文件：部分浏览器扩展会拦截 fetch（net::ERR_BLOCKED_BY_CLIENT），XHR 通常不受影响。
	 * 参见 /api/file/getFile：200 为文件体，202 为 JSON 错误。
	 */
	private getWorkspaceFileBlob(path: string): Promise<Blob> {
		return new Promise((resolve, reject) => {
			const xhr = new XMLHttpRequest();
			xhr.open("POST", "/api/file/getFile");
			xhr.setRequestHeader("Content-Type", "application/json");
			xhr.responseType = "blob";
			xhr.timeout = 300000;
			xhr.onload = () => {
				if (xhr.status === 200) {
					resolve(xhr.response);
					return;
				}
				if (xhr.status === 202 && xhr.response instanceof Blob) {
					xhr.response.text().then((t) => {
						try {
							const j = JSON.parse(t);
							reject(new Error(j?.msg || this.i18n.cannotgetfile));
						} catch {
							reject(new Error(this.i18n.cannotgetfile));
						}
					}, () => reject(new Error(this.i18n.cannotgetfile)));
					return;
				}
				reject(new Error(this.i18n.cannotgetfile));
			};
			xhr.onerror = () => reject(new Error(this.i18n.cannotgetfile));
			xhr.ontimeout = () => reject(new Error(this.i18n.cannotgetfile));
			xhr.send(JSON.stringify({ path }));
		});
	}

	/**
	 * 上传到远端并在思源内显示进度提示（基于 XHR upload progress）。
	 */
	private uploadWithProgress(
		url: string,
		formData: FormData,
		displayName: string,
		authHeader: string,
	): Promise<{ ok: boolean; body: string }> {
		return new Promise((resolve, reject) => {
			const xhr = new XMLHttpRequest();
			const progressId = `upload-progress-${Date.now()}-${Math.random().toString(36).slice(2)}`;
			xhr.open("POST", url);
			const auth = authHeader;
			if (auth) {
				xhr.setRequestHeader("Authorization", auth);
			}
			xhr.timeout = 300000;

			xhr.upload.onprogress = (evt) => {
				if (evt.lengthComputable && evt.total > 0) {
					const pct = Math.min(100, Math.round((evt.loaded / evt.total) * 100));
					showMessage(
						this.i18n.uploadProgress
							.replace("${name}", displayName)
							.replace("${percent}", `${pct}`),
						-1,
						"info",
						progressId,
					);
				} else {
					showMessage(
						this.i18n.uploading.replace("${name}", displayName),
						-1,
						"info",
						progressId,
					);
				}
			};

			xhr.onload = () => {
				const body = xhr.responseText || "";
				const doneText = this.i18n.uploadProgressDone.replace("${name}", displayName);
				showMessage(doneText, 1200, "info", progressId);
				resolve({ ok: xhr.status >= 200 && xhr.status < 300, body });
			};
			xhr.onerror = () => reject(new Error("upload request failed"));
			xhr.ontimeout = () => reject(new Error("upload request timeout"));
			xhr.send(formData);
		});
	}

	private async uploadFile(
		path: string,
		name: string,
		opts?: { menuElement?: HTMLElement; assetSrc?: string },
	): Promise<boolean> {
		try {
			const blob = await this.getWorkspaceFileBlob(path);
			const targets = this.getUploadTargets();
			if (targets.length === 0) {
				showMessage(this.noUploadTargetHint(), 6000, "error");
				return false;
			}

			const currentId = this.config.currentServerId as string;
			let anySuccess = false;
			let urlFromCurrent: string | null = null;
			let firstUrl: string | null = null;
			const failedLabels: string[] = [];

			for (const server of targets) {
				const base = this.normBaseURL(server);
				if (!base) {
					failedLabels.push(server.name || server.id);
					continue;
				}
				const displayName =
					targets.length > 1 ? `${name} (${server.name || base})` : name;
				const formData = new FormData();
				formData.append("title", name);
				formData.append("document", blob, name);
				formData.append("path", path);

				try {
					const { ok, body } = await this.uploadWithProgress(
						`${base}/api/documents/post_document/`,
						formData,
						displayName,
						this.getAuthHeader(server),
					);
					console.log("上传响应：", server.id, body);
					const { success, publicUrl } = this.parseUploadResult(body, ok, base);
					if (success) {
						anySuccess = true;
						if (publicUrl) {
							if (server.id === currentId) {
								urlFromCurrent = publicUrl;
							}
							if (!firstUrl) {
								firstUrl = publicUrl;
							}
						}
					} else {
						failedLabels.push(server.name || base);
					}
				} catch {
					failedLabels.push(server.name || base);
				}
			}

			const finalUrl = urlFromCurrent || firstUrl;

			if (!anySuccess) {
				showMessage(this.i18n.uploadFail.replace("${name}", name), 5000, "error");
				return false;
			}

			if (this.config.syncToAllServers && failedLabels.length > 0) {
				showMessage(
					((this.i18n as any).uploadPartialFail ?? "部分服务器上传失败：${servers}").replace(
						"${servers}",
						failedLabels.join(", "),
					),
					7000,
					"error",
				);
			}

			this.uploadedFiles.add(name);
			if (finalUrl) {
				this.imageMap.set(path, finalUrl);
			}

			if (opts?.menuElement && opts?.assetSrc && finalUrl) {
				await this.replaceEditorAssetWithUrl(opts.menuElement, opts.assetSrc, name, finalUrl);
			}
			return true;
		} catch (e) {
			console.error("上传异常：", e);
			showMessage(this.i18n.uploadError.replace("${name}", name), 5000, "error");
			return false;
		}
	}
}