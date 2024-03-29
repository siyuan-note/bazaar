import {
    App,
    ILuteNode,
    IObject,
    IOperation,
    IPosition,
    TTurnInto,
    TTurnIntoOne,
    TTurnIntoOneSub
} from "./../siyuan";

type TProtyleAction = "cb-get-append" | // 向下滚动加载
    "cb-get-before" | // 向上滚动加载
    "cb-get-unchangeid" | // 上下滚动，定位时不修改 blockid
    "cb-get-hl" | // 高亮
    "cb-get-focus" | // 光标定位
    "cb-get-focusfirst" | // 动态定位到第一个块
    "cb-get-setid" | // 重置 blockid
    "cb-get-all" | // 获取所有块
    "cb-get-backlink" | // 悬浮窗为传递型需展示上下文
    "cb-get-unundo" | // 不需要记录历史
    "cb-get-scroll" | // 滚动到指定位置
    "cb-get-context" | // 包含上下文
    "cb-get-html" | // 直接渲染，不需要再 /api/block/getDocInfo，否则搜索表格无法定位
    "cb-get-history" // 历史渲染

interface IToolbarItem {
    /** 唯一标示 */
    name: string;
    /** 提示 */
    tip?: string;
    /** svg 图标 */
    icon: string;
    /** 快捷键 */
    hotkey?: string;
    /** 提示位置 */
    tipPosition: string;
    click?(protyle: Protyle): void;
}

export interface IProtyleOption {
    action?: TProtyleAction[];
    mode?: "preview" | "wysiwyg";
    toolbar?: Array<string | IToolbarItem>;
    blockId?: string;
    key?: string;
    scrollAttr?: {
        rootId: string,
        startId: string,
        endId: string,
        scrollTop: number,
        focusId?: string,
        focusStart?: number,
        focusEnd?: number,
        zoomInId?: string,
    };
    defId?: string;
    render?: {
        background?: boolean,
        title?: boolean,
        gutter?: boolean,
        scroll?: boolean,
        breadcrumb?: boolean,
        breadcrumbDocName?: boolean,
    };
    typewriterMode?: boolean;

    after?(protyle: Protyle): void;
}

// REF: https://github.com/siyuan-note/siyuan/blob/dev/app/src/types/protyle.d.ts
export interface IProtyle {
    app: App;
    transactionTime: number;
    id: string;
    block: {
        id?: string;
        scroll?: boolean;
        parentID?: string;
        parent2ID?: string;
        rootID?: string;
        showAll?: boolean;
        mode?: number;
        blockCount?: number;
        action?: string[];
    };
    disabled: boolean;
    selectElement?: HTMLElement;
    ws?: any;
    notebookId?: string;
    path?: string;
    model?: any;
    updated: boolean;
    element: HTMLElement;
    scroll?: any;
    gutter?: any;
    breadcrumb?: {
        id: string;
        element: HTMLElement;
    };
    title?: {
        editElement: HTMLElement;
        element: HTMLElement;
    };
    background?: {
        element: HTMLElement;
        ial: Record<string, string>;
        iconElement: HTMLElement;
        imgElement: HTMLElement;
        tagsElement: HTMLElement;
        transparentData: string;
    };
    contentElement?: HTMLElement;
    options: any;
    lute?: Lute;
    toolbar?: Toolbar;
    preview?: any;
    hint?: any;
    upload?: any;
    undo?: any;
    wysiwyg?: any;
}

export class Protyle {

    public protyle: IProtyle;

    constructor(app: App, element: HTMLElement, options?: IProtyleOption)

    isUploading(): boolean

    destroy(): void

    resize(): void

    reload(focus: boolean): void

    /**
     * @param {boolean} [isBlock=false]
     * @param {boolean} [useProtyleRange=false]
     */
    insert(html: string, isBlock?: boolean, useProtyleRange?: boolean): void

    transaction(doOperations: IOperation[], undoOperations?: IOperation[]): void;

    /**
     * 多个块转换为一个块
     * @param {TTurnIntoOneSub} [subType] type 为 "BlocksMergeSuperBlock" 时必传
     */
    public turnIntoOneTransaction(selectsElement: Element[], type: TTurnIntoOne, subType?: TTurnIntoOneSub): void

    /**
     * 多个块转换
     * @param {Element} [nodeElement] 优先使用包含 protyle-wysiwyg--select 的块，否则使用 nodeElement 单块
     * @param type
     * @param {number} [subType] type 为 "Blocks2Hs" 时必传
     */
    public turnIntoTransaction(nodeElement: Element, type: TTurnInto, subType?: number): void

    public updateTransaction(id: string, newHTML: string, html: string): void

    public updateBatchTransaction(nodeElements: Element[], cb: (e: HTMLElement) => void): void

    public getRange(element: Element): Range

    public hasClosestBlock(element: Node): false | HTMLElement

    /**
     * @param {boolean} [toStart=true]
     */
    public focusBlock(element: Element, toStart?: boolean): false | Range
}

export class Toolbar {
    public element: HTMLElement;
    public subElement: HTMLElement;
    public subElementCloseCB: () => void;
    public range: Range;

    constructor(protyle: IProtyle)

    public render(protyle: IProtyle, range: Range, event?: KeyboardEvent)

    public showContent(protyle: IProtyle, range: Range, nodeElement: Element)

    public showWidget(protyle: IProtyle, nodeElement: HTMLElement, range: Range)

    public showTpl(protyle: IProtyle, nodeElement: HTMLElement, range: Range)

    public showCodeLanguage(protyle: IProtyle, languageElement: HTMLElement)

    public showRender(protyle: IProtyle, renderElement: Element, updateElements?: Element[], oldHTML?: string)

    public setInlineMark(protyle: IProtyle, type: string, action: "range" | "toolbar", textObj?: {
        color?: string,
        type: string
    })

    public getCurrentType(range: Range): string[]

    public showAssets(protyle: IProtyle, position: IPosition, avCB?: (url: string) => void)
}

export class Lute {
    public static WalkStop: number;
    public static WalkSkipChildren: number;
    public static WalkContinue: number;
    public static Version: string;
    public static Caret: string;

    public static New(): Lute;

    public static EChartsMindmapStr(text: string): string;

    public static NewNodeID(): string;

    public static Sanitize(html: string): string;

    public static EscapeHTMLStr(str: string): string;

    public static UnEscapeHTMLStr(str: string): string;

    public static GetHeadingID(node: ILuteNode): string;

    public static BlockDOM2Content(html: string): string;

    private constructor();

    public BlockDOM2Content(text: string): string;

    public BlockDOM2EscapeMarkerContent(text: string): string;

    public SetTextMark(enable: boolean): void;

    public SetHeadingID(enable: boolean): void;

    public SetProtyleMarkNetImg(enable: boolean): void;

    public SetSpellcheck(enable: boolean): void;

    public SetFileAnnotationRef(enable: boolean): void;

    public SetSetext(enable: boolean): void;

    public SetYamlFrontMatter(enable: boolean): void;

    public SetChineseParagraphBeginningSpace(enable: boolean): void;

    public SetRenderListStyle(enable: boolean): void;

    public SetImgPathAllowSpace(enable: boolean): void;

    public SetKramdownIAL(enable: boolean): void;

    public BlockDOM2Md(html: string): string;

    public BlockDOM2StdMd(html: string): string;

    public SetGitConflict(enable: boolean): void;

    public SetSuperBlock(enable: boolean): void;

    public SetTag(enable: boolean): void;

    public SetMark(enable: boolean): void;

    public SetSub(enable: boolean): void;

    public SetSup(enable: boolean): void;

    public SetBlockRef(enable: boolean): void;

    public SetSanitize(enable: boolean): void;

    public SetHeadingAnchor(enable: boolean): void;

    public SetImageLazyLoading(imagePath: string): void;

    public SetInlineMathAllowDigitAfterOpenMarker(enable: boolean): void;

    public SetToC(enable: boolean): void;

    public SetIndentCodeBlock(enable: boolean): void;

    public SetParagraphBeginningSpace(enable: boolean): void;

    public SetFootnotes(enable: boolean): void;

    public SetLinkRef(enalbe: boolean): void;

    public SetEmojiSite(emojiSite: string): void;

    public PutEmojis(emojis: IObject): void;

    public SpinBlockDOM(html: string): string;

    public Md2BlockDOM(html: string): string;

    public SetProtyleWYSIWYG(wysiwyg: boolean): void;

    public MarkdownStr(name: string, md: string): string;

    public IsValidLinkDest(text: string): boolean;

    public BlockDOM2InlineBlockDOM(html: string): string;

    public BlockDOM2HTML(html: string): string;
}
