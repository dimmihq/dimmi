/*
 * The loader's entry: reads the script tag that is running right now.
 */
import { load } from "../loader";

load(window, document, document.currentScript as HTMLScriptElement | null);
