/*
Package pipeline is the agent's machinery: a customer message comes
in, the gate decides whether it may answer, and the stages turn it
into a reply — or into an honest "I don't know" — with every step
traced.
*/
package pipeline
