/*
Package aidecision is the one AI decision step workflows, series and
conversation attributes share. It reads what it is given about a
conversation or a person and either chooses one of a few paths, with
no match as an answer of its own, or pulls typed values out of it:
text, a number, a date, yes or no, or one of a list of choices.

The decision model answers first and is asked again when it fails;
after that, or with none set up, the workspace's chat model is held to
a response schema, and that one tries each of the workspace's models in
turn. An answer that does not fit is asked again, never trimmed into
shape, and a value the models are not sure of is left out, so the
caller takes its fallback instead of acting on a guess.
*/
package aidecision
