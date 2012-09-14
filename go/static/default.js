function $(id) {
	if (document.all)
		return document.all[id];
	if (document.getElementById)
		return document.getElementById(id);
	for (var i=1; i<document.layers.length; i++) {
	    if (document.layers[i].id==id)
	      return document.layers[i];
	}
	return false;
}

var now = new Date();
if (now.getTimezoneOffset) document.cookie = "TZ=" + escape(now.getTimezoneOffset() * 60) + "; path=/";

var __rolloverCache = new Array();
var __rolloverOut;
var __rolloverOutSrc;

function rolloverInit(name, src) {
    __rolloverCache[name] = new Image();
    __rolloverCache[name].src = src;
}    

function rolloverOn(name, suffix)
{
    if (suffix) target = name + suffix;
    else target = name;
	if (document.images && __rolloverCache[name]) {
		__rolloverOut = target;
		__rolloverOutSrc = document.images[target].src;
		document.images[target].src = __rolloverCache[name].src;
	}
}

function rolloverOff()
{
	if (__rolloverOut && __rolloverOutSrc && document.images)	{
		document.images[__rolloverOut].src = __rolloverOutSrc;	
	}
}
