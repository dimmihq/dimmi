/*
Package whatsapp is the WhatsApp channel driver: replies sent through
Meta's Cloud API, webhook deliveries read into neutral shapes, their
signatures checked, Markdown converted both ways, and Meta's errors
mapped onto the channel contract. It talks to the Graph API itself, with
no SDK, over the install's HTTP client.
*/
package whatsapp
