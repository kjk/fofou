from google.appengine.ext import db
import main
import codecs, hashlib, os, os.path

def t1():
	entities = main.Topic.all().fetch(5)
	e = entities[0]
	s = e.key()
	print("key: " + str(e.key()))
	print("id : " + str(e.key().id()))

def touni(v):
	if not isinstance(v, basestring):
		return unicode(v)
	if isinstance(v, unicode): return v
	return unicode(v, "utf-8")

def kv(k, v):
	return u"%s: %s" % (touni(k), touni(v))

def sep(): return u""

def sertopic(e):
	lines = [
		#kv("K", e.key()),
		kv("I", e.key().id()),
		kv("F", e.forum.title),
		kv("S", e.subject),
		kv("On", e.created_on),
		kv("By", e.created_by),
		kv("D", e.is_deleted),
		sep(), sep()
	]
	return u"\n".join(lines)

def long2ip(val):
	slist = []
	for x in range(0,4):
		slist.append(str(int(val >> (24 - (x * 8)) & 0xFF)))
	return ".".join(slist)

def save_msg_sha1(msg):
	data = msg.encode("utf8")
	m = hashlib.sha1()
	m.update(data)
	sha1 = m.hexdigest()
	file_dir = "data/" + sha1[:2] + "/" + sha1[2:4]
	if not os.path.exists(file_dir):
		os.makedirs(file_dir)
	file_path = file_dir + "/" + sha1
	if not os.path.exists(file_path):
		open(file_path, "wb").write(data)
	return sha1

def serpost(e):
	msg = touni(e.message)
	sha1 = save_msg_sha1(msg)
	ip = e.user_ip_str
	if not ip or "" == ip:
		ip = long2ip(e.user_ip)
	lines = [
		kv("T", e.topic.key().id()),
		kv("M", sha1),
		kv("On", e.created_on),
		kv("D", e.is_deleted),
		kv("IP", ip),
		kv("UN", e.user_name),
		kv("UE", e.user_email),
		kv("UH", e.user_homepage),
		sep(), sep()
	]
	return u"\n".join(lines)

def topics(count=-1, batch_size=200, filename="topics.txt"):
	f = open(filename, "w")
	f.write(codecs.BOM_UTF8)
	entities = main.Topic.all().fetch(batch_size)
	n = 0
	while entities:
		s = u""
		for e in entities:
			s += sertopic(e)
			n += 1
			if n % 100 == 0:
				print("%d topics" % n)
		f.write(s.encode("utf8"))
		if count > 0 and n > count:
			break
		entities = main.Topic.all().filter('__key__ >', entities[-1].key()).fetch(batch_size)
	f.close()

def posts(count=-1, batch_size=400, filename="posts.txt"):
	f = open(filename, "w")
	f.write(codecs.BOM_UTF8)
	entities = main.Post.all().fetch(batch_size)
	n = 0
	while entities:
		s = u""
		for e in entities:
			s += serpost(e)
			n += 1
			if count > 0 and n > count:
				break
			if n % 100 == 0:
				print("%d posts" % n)
			f.write(s.encode("utf8"))
		entities = main.Post.all().filter('__key__ >', entities[-1].key()).fetch(batch_size)
	f.close()

