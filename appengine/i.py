from google.appengine.ext import db
import main
import codecs, hashlib, os, os.path

g_data_dir = "imported_data"

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

def long2ip(val):
	slist = []
	for x in range(0,4):
		slist.append(str(int(val >> (24 - (x * 8)) & 0xFF)))
	return ".".join(slist)

def create_dir(path):
	if not os.path.exists(path):
		os.makedirs(path)

def write_bin_to_file(path, data):
	open(path, "wb").write(data)

def write_str_to_file(path, data):
	write_bin_to_file(path, data.encode("utf8"))

def read_bin_from_file(path):
	if not os.path.exists(path): return None
	return open(path, "rb").read()

def read_str_from_file(path):
	return read_bin_from_file(path)

def save_msg_sha1(msg):
	data = msg.encode("utf8")

	m = hashlib.sha1()
	m.update(data)
	sha1 = m.hexdigest()

	file_dir = os.path.join(g_data_dir, "blobs", sha1[:2], sha1[2:4])
	create_dir(file_dir)
	file_path = os.path.join(file_dir, sha1)
	if not os.path.exists(file_path):
		write_bin_to_file(file_path, data)
	return sha1

def serforum(e):
	lines = [
		kv("I", e.key().id()),
		kv("U", e.url),
		kv("T", e.title),
		kv("TL", e.tagline),
		kv("D", e.is_disabled),
		kv("On", e.created_on),
		sep(), sep()
	]
	return u"\n".join(lines)

def forums(count=-1, batch_size=501):
	create_dir(g_data_dir)
	file_path = os.path.join(g_data_dir, "forums.txt")
	f = open(file_path, "wb")
	entities = main.Forum.all().fetch(batch_size)
	n = 0
	while entities:
		for e in entities:
			s = serforum(e)
			f.write(s.encode("utf8"))
		entities = main.Forum.all().filter('__key__ >', entities[-1].key()).fetch(batch_size)
	f.close()

def sertopic(e):
	lines = [
		kv("I", "%d.%d" % (e.forum.key().id(), e.key().id())),
		kv("S", e.subject),
		kv("On", e.created_on),
		kv("By", e.created_by),
		kv("D", e.is_deleted),
		sep(), sep()
	]
	return u"\n".join(lines)

def topics(count=-1, batch_size=501):
	create_dir(g_data_dir)
	file_path         = os.path.join(g_data_dir, "topics.txt")
	last_key_filepath = os.path.join(g_data_dir, "topics_last_key_id.txt")

	last_key_id = read_str_from_file(last_key_filepath)
	if None == last_key_id:
		print("Loading topics from the beginning")
		entities = main.Topic.all().fetch(batch_size)
	else:
		last_key = db.Key.from_path('Topic', long(last_key_id))
		print("Loading topics from key %s" % last_key_id)
		entities = main.Topic.all().filter('__key__ >', last_key).fetch(batch_size)
	last_key = None
	if len(entities) == 0:
		print("There are no new topics")

	f = open(file_path, "a")
	n = 0
	while entities:
		for e in entities:
			last_key = e.key()
			s = sertopic(e)
			f.write(s.encode("utf8"))
			n += 1
			if n % 100 == 0:
				print("%d topics" % n)
			if count > 0 and n >= count:
				entities = None
				break
		if entities is None:
			break
		entities = main.Topic.all().filter('__key__ >', last_key).fetch(batch_size)
	f.close()
	if last_key != None:
		print("New last topics key id: %d" % last_key.id())
		write_str_to_file(last_key_filepath, str(last_key.id()))

def serpost(e):
	msg = touni(e.message)
	sha1 = save_msg_sha1(msg)
	ip = e.user_ip_str
	if not ip or "" == ip:
		ip = long2ip(e.user_ip)
	lines = [
		kv("T", "%d.%d" % (e.forum.key().id(), e.topic.key().id())),
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

def posts(count=-1, batch_size=501):
	create_dir(g_data_dir)
	file_path         = os.path.join(g_data_dir, "posts.txt")
	last_key_filepath = os.path.join(g_data_dir, "posts_last_key_id.txt")

	last_key_id = read_str_from_file(last_key_filepath)
	if None == last_key_id:
		print("Loading posts from the beginning")
		entities = main.Post.all().fetch(batch_size)
	else:
		last_key = db.Key.from_path('Post', long(last_key_id))
		print("Loading posts from key %s" % last_key_id)
		entities = main.Post.all().filter('__key__ >', last_key).fetch(batch_size)
	last_key = None
	if len(entities) == 0:
		print("There are no new posts")

	f = open(file_path, "a")
	n = 0
	while entities:
		for e in entities:
			last_key = e.key()
			s = serpost(e)
			f.write(s.encode("utf8"))
			n += 1
			if count > 0 and n > count:
				break
			if n % 100 == 0:
				print("%d posts" % n)
			if count > 0 and n >= count:
				entities = None
				break
		if entities is None:
			break
		entities = main.Post.all().filter('__key__ >', last_key).fetch(batch_size)
	f.close()
	if last_key != None:
		print("New last poasts key id: %d" % last_key.id())
		write_str_to_file(last_key_filepath, str(last_key.id()))
