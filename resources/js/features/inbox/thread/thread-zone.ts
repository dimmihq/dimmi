/*
 * ThreadZone is the customer's time zone while the thread shows times in
 * it, and null while it shows them in the reader's.
 */
import { createContext } from "react";

export const ThreadZone = createContext<string | null>(null);
