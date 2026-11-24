/*
 * MentionContext hands the badges in a routine's instructions the
 * workspace's names as they load, so each follows a rename.
 */
import { createContext } from "react";

import { type Mentionables, nothingToMention } from "./mentions";

export const MentionContext = createContext<Mentionables>(nothingToMention);
