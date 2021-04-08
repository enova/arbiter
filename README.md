arbiter
-------

Arbiter is a tool to browse terraform outputs. It provides a web UI for browsing
through terraform state in directory format. It's designed to be able to work
with a selection of different named storage "backends". The main interface is a
configurable `http.Handler` so you can embed or run it as you deem necessary.
