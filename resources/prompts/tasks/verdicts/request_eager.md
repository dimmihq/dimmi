<!--
Added after request.md for chat rooms whose request sensitivity is "aggressive"
(app/ai/verdicts/requests.go). These teams would rather answer a message that needed nothing than
miss one that did. The room's confidence threshold is lower too, which the code handles.

It must only move the doubtful cases. Plain chatter must stay chatter, or the room fills with
conversations nobody asked for.
-->
## This channel would rather catch every request

The team in this channel prefers answering a message that needed nothing over missing one that did. When unsure whether a message is a `request` or something else, choose `request`. Keep `confidence` honest: a doubtful message is a low-confidence `request`. Messages that are plainly chatter, such as greetings, thanks, reactions and people talking among themselves, stay `chatter`.
