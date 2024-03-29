export * from "./types";

import {IProtyle, Lute, Protyle, Toolbar, IProtyleOption, TProtyleAction} from "./types/protyle";
import {IMenuBaseDetail} from "./types/events";
import {IGetDocInfo, IGetTreeStat} from "./types/response";

declare global {
    export interface Window extends Global {
    }
}

export type TEventBus = keyof IEventBusMap

export type TTurnIntoOne = "BlocksMergeSuperBlock" | "Blocks2ULs" | "Blocks2OLs" | "Blocks2TLs" | "Blocks2Blockquote"

export type TTurnIntoOneSub = "row" | "col"

export type TTurnInto = "Blocks2Ps" | "Blocks2Hs"

export type TCardType = "doc" | "notebook" | "all"

export type TOperation =
    "insert"
    | "update"
    | "delete"
    | "move"
    | "foldHeading"
    | "unfoldHeading"
    | "setAttrs"
    | "updateAttrs"
    | "append"
    | "insertAttrViewBlock"
    | "removeAttrViewBlock"
    | "addAttrViewCol"
    | "removeAttrViewCol"
    | "addFlashcards"
    | "removeFlashcards"
    | "updateAttrViewCell"
    | "updateAttrViewCol"
    | "sortAttrViewRow"
    | "sortAttrViewCol"
    | "setAttrViewColHidden"
    | "setAttrViewColWrap"
    | "setAttrViewColWidth"
    | "updateAttrViewColOptions"
    | "removeAttrViewColOption"
    | "updateAttrViewColOption"
    | "setAttrViewName"
    | "setAttrViewFilters"
    | "setAttrViewSorts"
    | "setAttrViewColCalc"
    | "updateAttrViewColNumberFormat"

export type TAVCol =
    "text"
    | "date"
    | "number"
    | "relation"
    | "rollup"
    | "select"
    | "block"
    | "mSelect"
    | "url"
    | "email"
    | "phone"

export interface Global {
    Lute: typeof Lute;
}

interface IKeymapItem {
    default: string,
    custom: string
}

export interface IKeymap {
    plugin: {
        [key: string]: {
            [key: string]: IKeymapItem
        }
    }
    general: {
        [key: string]: IKeymapItem
    }
    editor: {
        general: {
            [key: string]: IKeymapItem
        }
        insert: {
            [key: string]: IKeymapItem
        }
        heading: {
            [key: string]: IKeymapItem
        }
        list: {
            [key: string]: IKeymapItem
        }
        table: {
            [key: string]: IKeymapItem
        }
    }
}

export interface IEventBusMap {
    "click-flashcard-action": {
        card: ICard,
        type: string,   // 1 - 重来；2 - 困难；3 - 良好；4 - 简单；-1 - 显示答案；-2 - 上一个 ；-3 - 跳过
    };
    "click-blockicon": {
        menu: EventMenu,
        protyle: IProtyle,
        blockElements: HTMLElement[],
    };
    "click-editorcontent": {
        protyle: IProtyle,
        event: MouseEvent,
    };
    "click-editortitleicon": {
        menu: EventMenu,
        protyle: IProtyle,
        data: IGetDocInfo,
    };
    "click-pdf": {
        event: MouseEvent,
    };
    "destroy-protyle": {
        protyle: IProtyle,
    };
    "input-search": {
        protyle: Protyle,
        config: ISearchOption,
        searchElement: HTMLInputElement,
    };
    "loaded-protyle-dynamic": {
        protyle: IProtyle,
        positon: "afterend" | "beforebegin",
    };
    "loaded-protyle-static": {
        protyle: IProtyle,
    };
    "switch-protyle": {
        protyle: IProtyle,
    };
    "open-menu-av": IMenuBaseDetail & { selectRowElements: HTMLElement[] };
    "open-menu-blockref": IMenuBaseDetail;
    "open-menu-breadcrumbmore": {
        menu: EventMenu,
        protyle: IProtyle,
        data: IGetTreeStat,
    };
    "open-menu-content": IMenuBaseDetail & { range: Range };
    "open-menu-fileannotationref": IMenuBaseDetail;
    "open-menu-image": IMenuBaseDetail;
    "open-menu-link": IMenuBaseDetail;
    "open-menu-tag": IMenuBaseDetail;
    "open-menu-doctree": {
        menu: EventMenu,
        elements: NodeListOf<HTMLElement>,
        type: "doc" | "docs" | "notebook",
    };
    "open-menu-inbox": {
        menu: EventMenu,
        element: HTMLElement,
        ids: string[],
    };
    "open-noneditableblock": {
        protyle: IProtyle,
        toolbar: Toolbar,
    };
    "open-siyuan-url-block": {
        url: string,
        id: string,
        focus: boolean,
        exist: boolean,
    };
    "open-siyuan-url-plugin": {
        url: string,
    };
    "paste": {
        protyle: IProtyle,
        resolve: new <T>(value: T | PromiseLike<T>) => void,
        textHTML: string,
        textPlain: string,
        siyuanHTML: string,
        files: FileList | DataTransferItemList;
    }
    "ws-main": IWebSocketData;
    "sync-start": IWebSocketData;
    "sync-end": IWebSocketData;
    "sync-fail": IWebSocketData;
    "lock-screen": void;
    "mobile-keyboard-show": void;
    "mobile-keyboard-hide": void;
}

export interface IPosition {
    x: number;
    y: number;
    w?: number;
    h?: number;
    isLeft?: boolean;
}

export interface ITab {
    id: string;
    headElement: HTMLElement;
    panelElement: HTMLElement;
    model: IModel;
    title: string;
    icon: string;
    docIcon: string;
    updateTitle: (title: string) => void;
    pin: () => void;
    unpin: () => void;
    setDocIcon: (icon: string) => void;
    close: () => void;
}

export interface IModel {
    ws: WebSocket;
    app: App;
    reqId: number;
    parent: ITab | any;

    send(cmd: string, param: Record<string, unknown>, process?: boolean): void;
}

export interface ICustomModel extends IModel {
    tab: ITab;
    data: any;
    type: string;
    element: HTMLElement;

    init(): void;

    update?(): void;

    resize?(): void;

    beforeDestroy?(): void;

    destroy?(): void;
}

export interface IDockModel extends Omit<ICustomModel, "beforeDestroy"> {
}

export interface ITabModel extends ICustomModel {
}

export interface IObject {
    [key: string]: string;
}

export interface I18N {
    [key: string]: any;
}

export interface ILuteNode {
    TokensStr: () => string;
    __internal_object__: {
        Parent: {
            Type: number,
        },
        HeadingLevel: string,
    };
}

export interface ISearchOption {
    page?: number;
    removed?: boolean;  // 移除后需记录搜索内容 https://github.com/siyuan-note/siyuan/issues/7745
    name?: string;
    sort?: number;  //  0：按块类型（默认），1：按创建时间升序，2：按创建时间降序，3：按更新时间升序，4：按更新时间降序，5：按内容顺序（仅在按文档分组时），6：按相关度升序，7：按相关度降序
    group?: number;  // 0：不分组，1：按文档分组
    hasReplace?: boolean;
    method?: number; //  0：文本，1：查询语法，2：SQL，3：正则表达式
    hPath?: string;
    idPath?: string[];
    k: string;
    r?: string;
    types?: {
        mathBlock: boolean,
        table: boolean,
        blockquote: boolean,
        superBlock: boolean,
        paragraph: boolean,
        document: boolean,
        heading: boolean,
        list: boolean,
        listItem: boolean,
        codeBlock: boolean,
        htmlBlock: boolean,
        embedBlock: boolean,
        databaseBlock: boolean,
    };
}

export interface IWebSocketData {
    cmd: string;
    callback?: string;
    data: any;
    msg: string;
    code: number;
    sid: string;
}

export interface IPluginDockTab {
    position: "LeftTop" | "LeftBottom" | "RightTop" | "RightBottom" | "BottomLeft" | "BottomRight";
    size: {
        width: number,
        height: number,
    };
    icon: string;
    hotkey?: string;
    title: string;
    index?: number;
    show?: boolean;
}

export interface IMenuItemOption {
    iconClass?: string;
    label?: string;
    click?: (element: HTMLElement, event: MouseEvent) => boolean | void | Promise<boolean | void>;
    type?: "separator" | "submenu" | "readonly";
    accelerator?: string;
    action?: string;
    id?: string;
    submenu?: IMenuItemOption[];
    disabled?: boolean;
    icon?: string;
    iconHTML?: string;
    current?: boolean;
    bind?: (element: HTMLElement) => void;
    index?: number;
    element?: HTMLElement;
}

export interface ICommandOption {
    langKey: string // 用于区分不同快捷键的 key
    langText?: string // 快捷键功能描述文本
    /**
     * 目前需使用 MacOS 符号标识，顺序按照 ⌥⇧⌘，入 ⌥⇧⌘A
     * "Ctrl": "⌘",
     * "Shift": "⇧",
     * "Alt": "⌥",
     * "Tab": "⇥",
     * "Backspace": "⌫",
     * "Delete": "⌦",
     * "Enter": "↩",
     */
    hotkey: string,
    customHotkey?: string,
    callback?: () => void // 其余回调存在时将不会触
    globalCallback?: () => void // 焦点不在应用内时执行的回调
    fileTreeCallback?: (file: any) => void // 焦点在文档树上时执行的回调
    editorCallback?: (protyle: any) => void // 焦点在编辑器上时执行的回调
    dockCallback?: (element: HTMLElement) => void // 焦点在 dock 上时执行的回调
}

export interface IOperation {
    action: TOperation; // move， delete 不需要传 data
    id?: string;
    isTwoWay?: boolean; // 是否双向关联
    backRelationKeyID?: string; // 双向关联的目标关联列 ID
    avID?: string; // av
    format?: string; // updateAttrViewColNumberFormat 专享
    keyID?: string; // updateAttrViewCell 专享
    rowID?: string; // updateAttrViewCell 专享
    data?: any; // updateAttr 时为  { old: IObject, new: IObject }, updateAttrViewCell 时为 {TAVCol: {content: string}}
    parentID?: string;
    previousID?: string;
    retData?: any;
    nextID?: string; // insert 专享
    isDetached?: boolean; // insertAttrViewBlock 专享
    srcIDs?: string[]; // insertAttrViewBlock 专享
    name?: string; // addAttrViewCol 专享
    type?: TAVCol; // addAttrViewCol 专享
    deckID?: string; // add/removeFlashcards 专享
    blockIDs?: string[]; // add/removeFlashcards 专享
}

export interface ICard {
    deckID: string
    cardID: string
    blockID: string
    nextDues: IObject
}

export interface ICardData {
    cards: ICard[],
    unreviewedCount: number
    unreviewedNewCardCount: number
    unreviewedOldCardCount: number
}

export function fetchPost(url: string, data?: any, callback?: (response: IWebSocketData) => void, headers?: IObject): void;

export function fetchSyncPost(url: string, data?: any): Promise<IWebSocketData>;

export function fetchGet(url: string, callback: (response: IWebSocketData) => void): void;

export function openWindow(options: {
    position?: {
        x: number,
        y: number,
    },
    height?: number,
    width?: number,
    tab?: ITab,
    doc?: {
        id: string; // 块 id
    },
}): void;

export function openMobileFileById(app: App, id: string, action?: string[]): void;

export function openTab(options: {
    app: App,
    doc?: {
        id: string, // 块 id
        action?: TProtyleAction[],
        zoomIn?: boolean, // 是否缩放
    };
    pdf?: {
        path: string,
        page?: number,  // pdf 页码
        id?: string,    // File Annotation id
    };
    asset?: {
        path: string,
    };
    search?: ISearchOption;
    card?: {
        type: TCardType,
        id?: string, //  cardType 为 all 时不传，否则传文档或笔记本 id
        title?: string, //  cardType 为 all 时不传，否则传文档或笔记本名称
    };
    custom?: {
        id: string, // 插件名称+页签类型：plugin.name + tab.type
        icon: string,
        title: string,
        data?: any,
    };
    position?: "right" | "bottom";
    keepCursor?: boolean; // 是否跳转到新 tab 上
    removeCurrentTab?: boolean; // 在当前页签打开时需移除原有页签
    afterOpen?: () => void; // 打开后回调
}): Promise<ITab>

export function getFrontend(): "desktop" | "desktop-window" | "mobile" | "browser-desktop" | "browser-mobile";

export function lockScreen(app: App): void

export function getBackend(): "windows" | "linux" | "darwin" | "docker" | "android" | "ios";

export function adaptHotkey(hotkey: string): string;

export function confirm(title: string, text: string, confirmCallback?: (dialog: Dialog) => void, cancelCallback?: (dialog: Dialog) => void): void;

/**
 * @param timeout - ms. 0: manual close；-1: always show; 6000: default
 * @param {string} [type=info]
 */
export function showMessage(text: string, timeout?: number, type?: "info" | "error", id?: string): void;

export class App {
    plugins: Plugin[];
    appId: string
}

export abstract class Plugin {
    eventBus: EventBus;
    i18n: I18N;
    data: any;
    displayName: string;
    readonly name: string;
    app: App;
    commands: ICommandOption[];
    setting: Setting;
    protyleSlash: {
        filter: string[],
        html: string,
        id: string,
        callback(protyle: Protyle): void,
    }[];
    protyleOptions: IProtyleOption;

    constructor(options: {
        app: App,
        name: string,
        i18n: I18N,
    });

    onload(): void;

    onunload(): void;

    uninstall(): void;

    onLayoutReady(): void;

    /**
     * Must be executed before the synchronous function.
     * @param {string} [options.position=right]
     * @param {string} options.icon - Support svg id or svg tag.
     */
    addTopBar(options: {
        icon: string,
        title: string,
        callback: (event: MouseEvent) => void
        position?: "right" | "left"
    }): HTMLElement;

    /**
     * Must be executed before the synchronous function.
     * @param {string} [options.position=right]
     */
    addStatusBar(options: {
        element: HTMLElement,
        position?: "right" | "left",
    }): HTMLElement;

    openSetting(): void;

    loadData(storageName: string): Promise<any>;

    saveData(storageName: string, content: any): Promise<void>;

    removeData(storageName: string): Promise<any>;

    addIcons(svg: string): void;

    getOpenedTab(): { [key: string]: ICustomModel[] };

    /**
     * Must be executed before the synchronous function.
     */
    addTab(options: {
        type: string,
        beforeDestroy?: (this: ITabModel) => void,
        destroy?: (this: ITabModel) => void,
        resize?: (this: ITabModel) => void,
        update?: (this: ITabModel) => void,
        init: (this: ITabModel) => void,
    }): () => ITabModel;

    /**
     * Must be executed before the synchronous function.
     */
    addDock(options: {
        config: IPluginDockTab,
        data: any,
        type: string,
        destroy?: (this: IDockModel) => void,
        resize?: (this: IDockModel) => void,
        update?: (this: IDockModel) => void,
        init: (this: IDockModel, dock: IDockModel) => void,
    }): { config: IPluginDockTab, model: IDockModel };

    addCommand(options: ICommandOption): void;

    addFloatLayer(options: {
        ids: string[],
        defIds?: string[],
        x?: number,
        y?: number,
        targetElement?: HTMLElement
    }): void;

    updateCards(options: ICardData): Promise<ICardData> | ICardData;
}

export class Setting {
    constructor(options: {
        height?: string,
        width?: string,
        destroyCallback?: () => void,
        confirmCallback?: () => void,
    });

    addItem(options: {
        title: string,
        description?: string,
        actionElement?: HTMLElement,
        createActionElement?(): HTMLElement,
    }): void;

    open(name: string): void;
}

export class EventBus {
    on<
        K extends TEventBus,
        D = IEventBusMap[K],
    >(type: K, listener: (event: CustomEvent<D>) => any): void;

    once<
        K extends TEventBus,
        D = IEventBusMap[K],
    >(type: K, listener: (event: CustomEvent<D>) => any): void;

    off<
        K extends TEventBus,
        D = IEventBusMap[K],
    >(type: K, listener: (event: CustomEvent<D>) => any): void;

    emit<
        K extends TEventBus,
        D = IEventBusMap[K],
    >(type: K, detail?: D): boolean;
}

export class Dialog {

    element: HTMLElement;

    constructor(options: {
        title?: string,
        transparent?: boolean,
        content: string,
        width?: string
        height?: string,
        destroyCallback?: (options?: IObject) => void,
        disableClose?: boolean,
        disableAnimation?: boolean,
    });

    destroy(options?: IObject): void;

    bindInput(inputElement: HTMLInputElement | HTMLTextAreaElement, enterEvent?: () => void): void;
}

export class Menu {
    constructor(id?: string, closeCallback?: () => void);

    element: HTMLElement;

    showSubMenu(subMenuElement: HTMLElement): void;

    addItem(options: IMenuItemOption): HTMLElement;

    addSeparator(index?: number): void;

    open(options: { x: number, y: number, h?: number, w?: number, isLeft?: boolean }): void;

    /**
     * @param {string} [position=all]
     */
    fullscreen(position?: "bottom" | "all"): void;

    close(): void;
}

export class EventMenu {
    public menus: IMenuItemOption[];

    constructor();

    public addSeparator(index?: number): void;

    public addItem(menu: IMenuItemOption): void;
}
