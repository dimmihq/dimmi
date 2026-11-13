/*
Package toolset keeps an agent's tool definitions out of every model
call: a fixed short list plus load_tools and call_tool, with the rest
loaded on request, checked before they run and trimmed after.
*/
package toolset
