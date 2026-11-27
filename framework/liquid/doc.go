/*
Package liquid renders Liquid templates in a sandbox.

Templates come from people the server does not trust with code, such as
a customer's help center theme, so a render can only read the values it
is handed and write text. It never reaches a database, a file or the
network, and every render runs under limits: a number of steps, a time,
a size of output and a depth of partials. A template that loops forever
or nests partials without end stops with an error instead of holding the
server.

Output is escaped for HTML unless the value is HTML, which only the
server's own values, capture blocks (their own output already escaped)
and the raw filter make, so text a reader typed can never become markup.

The language follows Shopify's Liquid: output with filters, if, unless,
case, for with its options and forloop, assign, capture, render with
arguments, raw, comment and echo. Variables a template never received
read as nil; a filter or tag the engine does not know is an error when
the template is parsed, so a mistake shows before a theme goes live.
*/
package liquid
