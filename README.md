xcp
====

The cross application, network and os copy-paste w/ web-browser view

USAGE
=====

    xcp [<name>]
    -p <port>
    -v verbose

Use `-p` to specify a port number, and `-v` to turn off logging to stdout.

Start xcp on one machine then start on another machine, and copy paste to the xcp commandline or open a browser to http://localhost:<port>/<name> on a machine running xcp and view the contents in a web browser.

If multicast is setup then things will work across the network, otherwise a TCP host will need to be setup. The second server will need the same port and name to connect to directly. A browser can only be used between two or more connections.