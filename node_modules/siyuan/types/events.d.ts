import type {IProtyle} from "./protyle";
import {EventMenu} from "./../siyuan";

export interface IMenuBaseDetail {
    menu: EventMenu;
    protyle: IProtyle;
    element: HTMLElement;
}
