import {
    Plugin,
    showMessage,
    // confirm,
    // Dialog,
    Menu,
    // openTab,
    // adaptHotkey,
    getFrontend,
    // getBackend,
    // IModel,
    Protyle,
    // openWindow,
    // IOperation,
    Constants,
    // openMobileFileById,
    // lockScreen,
    // ICard,
    // ICardData
} from "siyuan";
import "@/index.scss";

const fs = require('fs')
// import fs from 'fs';
const path = require('path')
import axios from 'axios';
import JSZIP from 'jszip';
const axios_plus = axios.create({
    timeout: 10000,
    headers: {
        'Content-Type': 'application/json',
    },
});
import { SettingUtils } from "./libs/setting-utils";
// import { blob } from "stream/consumers";
import { pushErrMsg, pushMsg } from "./api";

const STORAGE_NAME = "menu-config";




interface Children {
    active: boolean;
    children: Children;
    docIcon: string;
    instance: string;
    pin: boolean;
    title: string;
    action: string;
    blockId: string;
    mode: string;
    notebookId: string;
    rootId: string;
}
interface IConfActivePage {
    children: Children[];
    height: string;
    instance: string;
    width: string;
}
interface exportHtmlData {
    content: string;
    id: string;
    name: string;
}

interface exportHtmlRootObject {
    code: number;
    msg: string;
    data: exportHtmlData;
}
interface IUploadArgsReq {
    appid: string;
    docid: string;
    content: string;
    version: string;
    theme: string;
    title: string
}
interface IGetLinkReq {
    appid: string;
    docid: string;
}
interface IAppearance {
    mode: number;
    modeOS: boolean;
    darkThemes: string[];
    lightThemes: string[];
    themeDark: string;
    themeLight: string;
    themeVer: string;
    icons: string[];
    icon: string;
    iconVer: string;
    codeBlockThemeLight: string;
    codeBlockThemeDark: string;
    lang: string;
    themeJS: boolean;
    closeButtonBehavior: number;
    hideStatusBar: boolean;
}
interface IConfSystem {
    id: string;
    name: string;
    kernelVersion: string;
    os: string;
    osPlatform: string;
    container: string;
    isMicrosoftStore: boolean;
    isInsider: boolean;
    homeDir: string;
    workspaceDir: string;
    appDir: string;
    confDir: string;
    dataDir: string;
    networkServe: boolean;
    networkProxy: INetworkProxy;
    uploadErrLog: boolean;
    disableGoogleAnalytics: boolean;
    downloadInstallPkg: boolean;
    autoLaunch: boolean;
    lockScreenMode: number;
}
interface INetworkProxy {
    scheme: string;
    host: string;
    port: string;
}
interface Ial {
    id: string;
    title: string;
    type: string;
    updated: string;
}

interface AttrView {
    id: string;
    name: string;
}

interface IgetDocResData {
    id: string;
    rootID: string;
    name: string;
    refCount: number;
    subFileCount: number;
    refIDs: any[];
    ial: Ial;
    icon: string;
    attrViews: AttrView[];
}

interface IgetDocRes {
    code: number;
    msg: string;
    data: IgetDocResData;
}
interface IRes {
    err: number;
    msg: string;
    data: string;
}
interface IFuncData {
    err: boolean,
    fdata: string,
}

export default class PluginSample extends Plugin {

    // private customTab: () => IModel;
    private isMobile: boolean;
    private settingUtils: SettingUtils;

    async onload() {
        this.data[STORAGE_NAME] = { readonlyText: "Readonly" };

        console.debug("loading plugin-sample", this.i18n);

        const frontEnd = getFrontend();
        this.isMobile = frontEnd === "mobile" || frontEnd === "browser-mobile";

        // 在mac 上使用SF symbol生成
        this.addIcons(`<symbol id="iconFace" viewBox="0 0 32 32">
        <g>
        <rect height="37.0645" opacity="0" width="24.623" x="0" y="0"/>
        <path d="M24.2676 15.0664L24.2676 28.7109C24.2676 31.5273 22.832 32.9492 19.9746 32.9492L4.29297 32.9492C1.43555 32.9492 0 31.5273 0 28.7109L0 15.0664C0 12.25 1.43555 10.8281 4.29297 10.8281L8.4082 10.8281L8.4082 13.0293L4.32031 13.0293C2.95312 13.0293 2.20117 13.7676 2.20117 15.1895L2.20117 28.5879C2.20117 30.0098 2.95312 30.748 4.32031 30.748L19.9336 30.748C21.2871 30.748 22.0664 30.0098 22.0664 28.5879L22.0664 15.1895C22.0664 13.7676 21.2871 13.0293 19.9336 13.0293L15.8594 13.0293L15.8594 10.8281L19.9746 10.8281C22.832 10.8281 24.2676 12.25 24.2676 15.0664Z" fill="#000000" fill-opacity="0.85"/>
        <path d="M12.127 22.2441C12.7148 22.2441 13.2207 21.752 13.2207 21.1777L13.2207 7.13672L13.1387 5.08594L14.0547 6.05664L16.1328 8.27148C16.3242 8.49023 16.5977 8.59961 16.8711 8.59961C17.4316 8.59961 17.8691 8.18945 17.8691 7.62891C17.8691 7.3418 17.7461 7.12305 17.541 6.91797L12.9199 2.46094C12.6465 2.1875 12.4141 2.0918 12.127 2.0918C11.8535 2.0918 11.6211 2.1875 11.334 2.46094L6.71289 6.91797C6.50781 7.12305 6.39844 7.3418 6.39844 7.62891C6.39844 8.18945 6.80859 8.59961 7.38281 8.59961C7.64258 8.59961 7.94336 8.49023 8.13477 8.27148L10.1992 6.05664L11.1289 5.08594L11.0469 7.13672L11.0469 21.1777C11.0469 21.752 11.5391 22.2441 12.127 22.2441Z" fill="#000000" fill-opacity="0.85"/>
       </g>
        </symbol>
<symbol id="iconSaving" viewBox="0 0 32 32">
<path d="M224.653061 642.612245c-72.097959 0-130.612245-58.514286-130.612245-130.612245s58.514286-130.612245 130.612245-130.612245 130.612245 58.514286 130.612245 130.612245-58.514286 130.612245-130.612245 130.612245z m0-219.428572c-49.110204 0-88.816327 39.706122-88.816326 88.816327s39.706122 88.816327 88.816326 88.816327 88.816327-39.706122 88.816327-88.816327-40.228571-88.816327-88.816327-88.816327zM580.440816 355.265306c-72.097959 0-130.612245-58.514286-130.612245-130.612245s58.514286-130.612245 130.612245-130.612245 130.612245 58.514286 130.612245 130.612245-59.036735 130.612245-130.612245 130.612245z m0-219.428571c-49.110204 0-88.816327 39.706122-88.816326 88.816326s39.706122 88.816327 88.816326 88.816327 88.816327-39.706122 88.816327-88.816327-40.228571-88.816327-88.816327-88.816326zM799.346939 929.959184c-72.097959 0-130.612245-58.514286-130.612245-130.612245s58.514286-130.612245 130.612245-130.612245 130.612245 58.514286 130.612245 130.612245-58.514286 130.612245-130.612245 130.612245z m0-219.428572c-49.110204 0-88.816327 39.706122-88.816327 88.816327s39.706122 88.816327 88.816327 88.816326 88.816327-39.706122 88.816326-88.816326-39.706122-88.816327-88.816326-88.816327z" fill="#13227a" p-id="4434"></path><path d="M301.453061 454.530612c-6.791837 0-13.583673-3.134694-17.763265-9.404081-6.269388-9.926531-3.657143-22.465306 6.269388-28.734694l201.665306-131.657143c9.926531-6.269388 22.465306-3.657143 28.734694 6.269388s3.657143 22.465306-6.269388 28.734694L312.42449 451.395918c-3.134694 2.089796-7.314286 3.134694-10.971429 3.134694zM699.036735 775.836735c-3.134694 0-6.791837-0.522449-9.404082-2.612245l-376.163265-195.395919c-10.44898-5.22449-14.106122-17.763265-8.881633-28.212244 5.22449-10.44898 17.763265-14.106122 28.212245-8.881633l376.163265 195.395918c10.44898 5.22449 14.106122 17.763265 8.881633 28.212245-3.657143 7.314286-10.971429 11.493878-18.808163 11.493878z" fill="#13227a" p-id="4435"></path><</symbol>`);

        // 添加顶部菜单
        const topBarElement = this.addTopBar({
            icon: "iconFace",
            title: this.i18n.addTopBarIcon,
            position: "right",
            callback: () => {
                if (this.isMobile) {
                    this.addMenu();
                } else {
                    let rect = topBarElement.getBoundingClientRect();
                    // 如果被隐藏，则使用更多按钮
                    if (rect.width === 0) {
                        rect = document.querySelector("#barMore").getBoundingClientRect();
                    }
                    if (rect.width === 0) {
                        rect = document.querySelector("#barPlugins").getBoundingClientRect();
                    }
                    this.addMenu(rect);
                }
            }
        });

        // 当onLayoutReady()执行时，this.settingUtils被载入
        this.settingUtils = new SettingUtils(this, STORAGE_NAME);

        try {
            this.settingUtils.load();
        } catch (error) {
            console.error("Error loading settings storage, probably empty config json:", error);
        }
        this.settingUtils.addItem({
            key: "share_link",
            value: "",
            type: "textinput",
            title: this.i18n.memu_share_link_title,
            description: "",
        });


        this.settingUtils.addItem({
            key: "create_share",
            value: "",
            type: "button",
            title: this.i18n.menu_create_share_title,
            description: this.i18n.menu_create_share_desc,
            button: {
                label: this.i18n.menu_create_share_label,
                callback: async () => {

                    let g = await this.createLink()
                    if (g.err == false) {
                        this.settingUtils.set("share_link", g.fdata)
                        pushMsg("创建成功",7000)
                    } else {
                        pushErrMsg(g.fdata, 7000)
                    }
                }
            }
        });
        this.settingUtils.addItem({
            key: "delete_share",
            value: "",
            type: "button",
            title: this.i18n.menu_delete_share_title,
            description: "",
            button: {
                label: this.i18n.menu_delete_share_label,
                callback: async () => {
                    let g = await this.deleteLink()
                    if (g.err == false) {
                        this.settingUtils.set("share_link", "");
                    }

                }
            }
        });
        this.settingUtils.addItem({
            key: "address",
            value: "http://124.223.15.220",
            type: "textinput",
            title: this.i18n.menu_address_title,
            description: this.i18n.menu_address_desc,
        });

        this.settingUtils.addItem({
            key: "access_code",
            value: "",
            type: "textinput",
            title: this.i18n.memu_access_code_title,
            description: this.i18n.memu_access_code_desc,
        });

        this.settingUtils.addItem({
            key: "Hint",
            value: "",
            type: "hint",
            title: this.i18n.hintTitle,
            description: this.i18n.hintDesc,
        });

        this.protyleSlash = [{
            filter: ["insert emoji 😊", "插入表情 😊", "crbqwx"],
            html: `<div class="b3-list-item__first"><span class="b3-list-item__text">${this.i18n.insertEmoji}</span><span class="b3-list-item__meta">😊</span></div>`,
            id: "insertEmoji",
            callback(protyle: Protyle) {
                protyle.insert("😊");
            }
        }];

        this.protyleOptions = {
            toolbar: ["block-ref",
                "a",
                "|",
                "text",
                "strong",
                "em",
                "u",
                "s",
                "mark",
                "sup",
                "sub",
                "clear",
                "|",
                "code",
                "kbd",
                "tag",
                "inline-math",
                "inline-memo",
                "|",
                {
                    name: "insert-smail-emoji",
                    icon: "iconEmoji",
                    hotkey: "⇧⌘I",
                    tipPosition: "n",
                    tip: this.i18n.insertEmoji,
                    click(protyle: Protyle) {
                        protyle.insert("😊");
                    }
                }],
        };

        console.debug(this.i18n.helloPlugin);
    }

    onLayoutReady() {
        console.debug("加载插件")
        this.settingUtils.load();
    }

    async onunload() {

        console.debug(this.i18n.byePlugin);
        await this.settingUtils.save();
        showMessage("Goodbye SiYuan Plugin");
        console.debug("onunload");
    }

    uninstall() {
        console.debug("uninstall");
    }

    async getsystemInfo() {
        // 获取当前页的ID
        const url = "api/system/getConf"

        let data = "{}"
        let config_system: IConfSystem = {
            id: "",
            name: "",
            kernelVersion: "",
            os: "",
            osPlatform: "",
            container: "",
            isMicrosoftStore: false,
            isInsider: false,
            homeDir: "",
            workspaceDir: "",
            appDir: "",
            confDir: "",
            dataDir: "",
            networkServe: false,
            networkProxy: {
                scheme: "",
                host: "",
                port: ""
            },
            uploadErrLog: false,
            disableGoogleAnalytics: false,
            downloadInstallPkg: false,
            autoLaunch: false,
            lockScreenMode: 0
        }

        // 设置handle
        let headers = {}
        const access_code = this.settingUtils.get("access_code")
        if (access_code == "") {
            headers = {
                'Content-Type': 'application/json'
            };
        } else {
            headers = {
                'Authorization': ' Token ' + access_code,
                'Content-Type': 'application/json'
            };
        }

        return axios.post(url, data, headers)
            .then(function (response) {
                config_system = response.data.data.conf.system
                return config_system

            })
            .catch(function (error) {

                console.error(error);
                return config_system
            });
    }
    async getSystemID() {
        let system_info = await this.getsystemInfo()
        return system_info.id
    }
    async getDocTitle(id) {
        const url = "api/block/getDocInfo"
        let data = {
            id: id
        }
        let res: IgetDocRes = {
            code: 0,
            msg: "",
            data: {
                id: "",
                rootID: "",
                name: "",
                refCount: 0,
                subFileCount: 0,
                refIDs: [],
                ial: {
                    id: "",
                    title: "",
                    type: "",
                    updated: ""
                },
                icon: "",
                attrViews: []
            }
        }

        // 设置headers
        let headers = {}
        const access_code = this.settingUtils.get("access_code")
        if (access_code == "") {
            headers = {
                'Content-Type': 'application/json'
            };
        } else {
            headers = {
                'Authorization': ' Token ' + access_code,
                'Content-Type': 'application/json'
            };
        }

        return axios_plus.post(url, data, headers)
            .then(function (response) {
                res = response.data
                return res.data.name

            })
            .catch(function (error) {
                console.error(error);
                return ""
            });

    }
    async getActivePage() {
        // 获取当前页的ID
        const url = "api/system/getConf"

        let data = "{}"
        let active_page_list: IConfActivePage = {
            children: [],
            height: "",
            instance: "",
            width: ""
        }
        // 设置headers
        let headers = {}
        const access_code = this.settingUtils.get("access_code")
        if (access_code == "") {
            headers = {
                'Content-Type': 'application/json'
            };
        } else {
            headers = {
                'Authorization': ' Token ' + access_code,
                'Content-Type': 'application/json'
            };
        }

        return axios_plus.post(url, data, headers)
            .then(function (response) {
                active_page_list = response.data.data.conf.uiLayout.layout.children[0].children[1].children[0]

                for (let i = 0; i < active_page_list.children.length; i++) {
                    if (active_page_list.children[i].active == true) {
                        return active_page_list.children[i].children.blockId
                    }
                }

            })
            .catch(function (error) {

                console.error(error);
                return ""
            });
    }
    async getTheme() {
        const url = "api/system/getConf"

        let data = "{}"
        let res_data: IAppearance = {
            mode: 0,
            modeOS: false,
            darkThemes: [],
            lightThemes: [],
            themeDark: "",
            themeLight: "",
            themeVer: "",
            icons: [],
            icon: "",
            iconVer: "",
            codeBlockThemeLight: "",
            codeBlockThemeDark: "",
            lang: "",
            themeJS: false,
            closeButtonBehavior: 0,
            hideStatusBar: false
        }

        // 设置headers
        let headers = {}
        const access_code = this.settingUtils.get("access_code")
        if (access_code == "") {
            headers = {
                'Content-Type': 'application/json'
            };
        } else {
            headers = {
                'Authorization': ' Token ' + access_code,
                'Content-Type': 'application/json'
            };
        }

        return axios_plus.post(url, data, headers)
            .then(function (response) {
                res_data = response.data.data.conf.appearance
                if (res_data.mode == 0) {
                    return res_data.themeLight

                } else {
                    return res_data.themeDark
                }

            })
            .catch(function (error) {
                console.error(error);
                return ""
            });
    }

    // 功能: 导出html
    // 输入: 页面ID
    // 输入: 保存路径
    async exportHtml(id, savePath) {
        let url = "api/export/exportHTML"
        let data = {
            id: id,
            pdf: false,
            savePath: savePath
        }
        let res_data: exportHtmlRootObject = {
            code: 0,
            msg: "",
            data: {
                content: "",
                id: "",
                name: ""
            }
        }
        // 设置headers
        let headers = {}
        const access_code = this.settingUtils.get("access_code")
        if (access_code == "") {
            headers = {
                'Content-Type': 'application/json'
            };
        } else {
            headers = {
                'Authorization': 'Token ' + access_code,
                'Content-Type': 'application/json'
            };
        }
        console.debug(headers)
        return axios_plus.post(url, data, headers)
            .then(function (response) {
                res_data = response.data
                if (res_data.code == 0 && res_data.data.id == id) {
                    console.debug("导出成功")
                    return res_data.data.content
                } else {
                    return ""
                }
            })
            .catch(function (error) {
                console.error(error);
                return ""

            });

    }

    async compressFolder(targetDir: string, outputFile: string) {
        let zip = new JSZIP();
        let list = this.recurseDirectory(targetDir)
        for (let i = 0; i < list.length; i++) {
            let filePath = list[i]
            let fileName = filePath.replace(targetDir, "")
            zip.file(fileName, fs.readFileSync(filePath))
        }

        zip.generateAsync({//设置压缩格式，开始打包
            type: "nodebuffer",//nodejs用
            compression: "DEFLATE",//压缩算法
            compressionOptions: {//压缩级别
                level: 9
            }
        }).then(function (content) {
            fs.writeFileSync(outputFile, content, "utf-8");//将打包的内容写入 当前目录下的 result.zip中
        });

    }

    recurseDirectory(directoryPath: String) {
        const files: String[] = [];
        // 获取文件夹中的所有文件和文件夹
        const entries = fs.readdirSync(directoryPath, { withFileTypes: true });


        for (const entry of entries) {
            const entryPath = path.join(directoryPath, entry.name);

            if (entry.isFile()) {
                files.push(entryPath);
                // console.debug("文件:",directoryPath+"/"+entry.name)
            }
            else if (entry.isDirectory()) {
                // 递归文件夹并将其添加到文件数组中
                files.push(...this.recurseDirectory(entryPath));
            }
        }

        return files;
    }

    // 功能: 上传导出的html文件夹的压缩包到分享服务器
    // 参数: serverAddress 表示服务器地址
    // 参数: dir 表示需要压缩的html文件夹路径
    // 返回参数: IFuncData.err 表示请求是否成功
    // 返回参数: IFuncData.data 表示返回消息
    async uploadFile(serverAddress, dir, appid, docid) {


        const zip_file = dir + ".zip"
        this.compressFolder(dir, zip_file)

        serverAddress = serverAddress + '/api/upload_file' + `?appid=${appid}&docid=${docid}`

        const formData = new FormData();


        var myBlob = new Blob([fs.readFileSync(zip_file)], { type: "text/zip" });
        formData.append('file', myBlob);

        var headers = {
            'Content-Type': 'multipart/form-data',
        }
        console.debug(`上传文件 文件地址:${zip_file} 后台地址:${serverAddress}`)
        // 发送请求

        let g: IFuncData = {
            err: true,
            fdata: ""
        }
        return axios.post(serverAddress, formData, { headers, timeout: 300000, decompress: false })
            .then(function (response) {
                let data: IRes = response.data
                console.debug(response.data)
                if (data.err == 0) {
                    g.err = false
                    g.fdata = data.data
                    return g
                } else {
                    g.fdata = data.msg
                    return g
                }
            })
            .catch(function (error) {
                g.err = false
                g.fdata = error
                console.error(error)
                return g
            })
    }

    // 功能: 上传分享文档的参数到分享服务器
    // 返回参数: IFuncData.err 表示请求是否成功
    // 返回参数: IFuncData.data 表示返回链接
    async uploadArgs(server_address: String, data: IUploadArgsReq) {
        let url = server_address + "/api/upload_args"
        console.debug(`${this.i18n.log_upload_address_desc}:${url}\nappid:${data.appid}\ndocid:${data.docid}\nversion:${data.version}\ntheme:${data.theme}\ntitle:${data.title}`)

        let g: IFuncData = {
            err: true,
            fdata: ""
        }
        return axios_plus.post(url, data)
            .then(function (response) {
                let data: IRes = response.data
                g.err = false
                g.fdata = data.data
                console.debug(data)
                if (data.err == 0) {
                    return g
                } else {
                    pushErrMsg(data.msg, 7000)
                    return g
                }
            })
            .catch(function (error) {
                console.error(error)
                g.fdata = this.i18n.err_upload
                g.err = true
                return g
            })
    }

    // 功能: 获取分享链接
    // 返回参数: IFuncData.err 表示请求是否成功
    // 返回参数: IFuncData.data 表示返回链接
    async createLink() {
        let savePath: String
        let docid = await this.getActivePage()

        let system_info = await this.getsystemInfo()
        // 如果是mac
        if (system_info.os == "darwin") {
            savePath = "/tmp"
        } else if (system_info.os == "win32") {
            savePath = system_info.homeDir + "\\AppData\\Local\\Temp"
        } else if (system_info.os == "linux") {
            savePath = "/tmp"
        }
        // 获取用户名

        savePath = system_info.dataDir + "/tmp_share/" + docid
        let content = await this.exportHtml(docid, savePath)
        if (content == "") {
            return
        }

        let data: IUploadArgsReq = {
            appid: await this.getSystemID(),
            docid: docid,
            content: content,
            version: Constants.SIYUAN_VERSION,
            theme: await this.getTheme(),
            title: await this.getDocTitle(docid)
        };

        let g: IFuncData = {
            err: true,
            fdata: ""
        }
        let server_address = this.settingUtils.get("address");
        g = await this.uploadFile(server_address, savePath, data.appid, data.docid)
        if (g.err == true) {
            return g
        }
        g = await this.uploadArgs(server_address, data)
        return g
    }

    // 功能: 获取分享链接
    // 返回: IFuncData结构体，包含err和data，
    // 返回参数: err 表示请求是否成功
    // 返回参数: data 表示返回链接
    async getLink() {
        const data: IGetLinkReq = {
            appid: await this.getSystemID(),
            docid: await this.getActivePage(),
        };
        const url = this.settingUtils.get("address") + "/api/getlink"


        console.debug(`${this.i18n.log_upload_address_desc}:${url}\nappid:${data.appid}\ndocid:${data.docid}`)
        return axios_plus.post(url, data)
            .then(function (response) {
                let data: IRes = response.data
                console.debug(data)

                let g: IFuncData = {
                    err: false,
                    fdata: data.data
                }

                if (data.err != 0) {
                    g.err = true
                }
                return g

            })
            .catch(function (error) {
                var g: IFuncData = {
                    err: true,
                    fdata: ""
                }
                console.error(error)
                pushErrMsg(this.i18n.err_upload, 7000)
                return g
            })
    }

    // 功能: 删除分享链接
    // 返回: IFuncData结构体，包含err和data，
    // 返回参数: err 表示请求是否成功
    async deleteLink() {
        const data: IGetLinkReq = {
            appid: await this.getSystemID(),
            docid: await this.getActivePage(),
        };
        const url = this.settingUtils.get("address") + "/api/deletelink"

        console.debug(`${this.i18n.log_upload_address_desc}:${url}\nappid:${data.appid}\ndocid:${data.docid}`)
        return axios_plus.post(url, data)
            .then(function (response) {
                let data: IRes = response.data
                console.debug(data)

                let g: IFuncData = {
                    err: false,
                    fdata: data.data
                }

                if (data.err != 0) {
                    g.err = true
                }
                return g

            })
            .catch(function (error) {
                var g: IFuncData = {
                    err: true,
                    fdata: ""
                }
                console.error(error)
                pushErrMsg(this.i18n.err_upload, 7000)
                return g
            })
    }

    // 插件菜单列表
    private async addMenu(rect?: DOMRect) {
        const menu = new Menu("topBarSample", () => {
            console.debug(this.i18n.byeMenu);
        });

        menu.addSeparator();
        menu.addItem({
            icon: "iconSettings",
            label: "分享设置",
            click: async () => {
                console.debug("打开设置")
                let g = await this.getLink()
                if (g.err == false) {
                    this.settingUtils.set("share_link", g.fdata)
                }
                this.openSetting();
            }
        });
        menu.addSeparator();


        if (this.isMobile) {
            menu.fullscreen();
        } else {
            menu.open({
                x: rect.right,
                y: rect.bottom,
                isLeft: true,
            });
        }
    }
}
