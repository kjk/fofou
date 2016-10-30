## Get the code

git clone --recursive https://github.com/kjk/fofou.git

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

You probably want to run it on a server (I use Ubuntu) but when testing you can run it on Mac.

You need to create `config.json` (see `sample_config.json` for example).

Since login system uses Twitter OAuth, you need to get token and secret from https://dev.twitter.com/ and set AdminTwitterUser to your Twitter handle (this is the user who is the admin of the forum).

To ensure encryption of cookies, you need to set random CookieAuthKeyHexStr and CookieEncrKeyHexStr. The easies way is to leave them blank and new random values will be printed to stdout.

Look at `scripts/run.sh` to see how to compile and run the forum.

## Deployment

When you want to run the code in production, you probably want to deploy it to a server.

You can take a look at `fabfile.py` (Fabric deployment script) for an example
on how to do automate deployments.

## Design philosophy

You'll quickly see that Fofou differs in many ways from most common forum
software. There are good reasons for the differences and Joel Spolsky describes
those reason in great detail:
http://www.joelonsoftware.com/articles/BuildingCommunitieswithSo.html

## License

The Go code is written completely by me and is in Public Domain.

Html/css/js files are mostly lifted from FuitShow, so they fall under
FruitShow's BSD license.
