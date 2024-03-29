import {IObject} from "../siyuan";

export interface IGetDocInfo {
    ial: IObject;
    icon: string;
    id: string;
    name: string;
    refCount: number;
    refIDs: string[];
    rootID: string;
    subFileCount: number;
}

export interface IGetTreeStat {
    imageCount: number;
    linkCount: number;
    refCount: number;
    runeCount: number;
    wordCount: number;
}
