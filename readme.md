## Overview

For more info: http://blog.kowalczyk.info/software/fofou/

Fofou (Forums For You) is a simple forum software inspired by
Joel On Software forum software (http://discuss.joelonsoftware.com/?joel).

It's mostly a port of FruitShow PHP forum (http://sourceforge.net/projects/fruitshow).

This is a version written in Go. There's also a version in Python for
App Engine: https://github.com/kjk/fofou_appengine

## Where can I see it in action?

Forums for my Sumatra PDF reader are powered by Fofou:
http://forums.fofou.org/sumatrapdf/

## Installation

You need to compile the code and run on a server. I use Linux (Ubuntu).

You can take a look at fabfile.py (Fabric deployment script) for an example
on how to do it.

You need to modify/add forum definition in forums directory.

## Design philosophy

You'll quickly see that Fofou differs in many ways from most common forum
software. There are good reasons for the differences and Joel Spolsky describes
those reason in great detail:
http://www.joelonsoftware.com/articles/BuildingCommunitieswithSo.html

## License

The Go code is written completely by me and is in Public Domain.

Html/css/js files are mostly lifted from FuitShow, so they fall under
FruitShow's BSD license.
