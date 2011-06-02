# This code is in Public Domain. Take all the code you want, we'll just write more.
import os, string, Cookie, sha, time, random, cgi, urllib, datetime, StringIO, pickle
import wsgiref.handlers
from google.appengine.api import users
from google.appengine.api import memcache
from google.appengine.ext import webapp
from google.appengine.ext import db
from google.appengine.ext.webapp import template
from django.utils import feedgenerator
from django.template import Context, Template
import logging
from offsets import *

# Structure of urls:
#
# Top-level urls
#
# / - list of all forums
#
# /manageforums[?forum=<key> - edit/create/disable forums
#
# Per-forum urls
#
# /<forum_url>/[?from=<n>]
#    index, lists of topics, optionally starting from topic <n>
#
# /<forum_url>/post[?id=<id>]
#    form for creating a new post. if "topic" is present, it's a post in
#    existing topic, otherwise a post starting a new topic
#
# /<forum_url>/topic?id=<id>&comments=<comments>
#    shows posts in a given topic, 'comments' is ignored (just a trick to re-use
#    browser's history to see if the topic has posts that user didn't see yet
#
# /<forum_url>/postdel?<post_id>
# /<forum_url>/postundel?<post_id>
#    delete/undelete post
#
# /<forum_url>/rss
#    rss feed for first post in the topic (default)
#
# /<forum_url>/rssall
#    rss feed for all posts

# HTTP codes
HTTP_NOT_ACCEPTABLE = 406
HTTP_NOT_FOUND = 404

RSS_MEMCACHED_KEY = "rss"

def rss_memcache_key(forum):
    return RSS_MEMCACHED_KEY + str(forum.key().id)

BANNED_IPS = {
    "59.181.121.8"  : 1,
    "62.162.98.194" : 1,
    "93.191.0.129"  : 1,
    #"127.0.0.1" : 1,
}

def my_hostname():
    # TODO: handle https as well
    h = "http://" + os.environ["SERVER_NAME"];
    port = os.environ["SERVER_PORT"]
    if port != "80":
        h += ":%s" % port
    return h

class FofouUser(db.Model):
  # according to docs UserProperty() cannot be optional, so for anon users
  # we set it to value returned by anonUser() function
  # user is uniquely identified by either user property (if not equal to
  # anonUser()) or cookie
  user = db.UserProperty()
  cookie = db.StringProperty()
  # email, as entered in the post form, can be empty string
  email = db.StringProperty()
  # name, as entered in the post form
  name = db.StringProperty()
  # homepage - as entered in the post form, can be empty string
  homepage = db.StringProperty()
  # value of 'remember_me' checkbox selected during most recent post
  remember_me = db.BooleanProperty(default=True)

class Forum(db.Model):
  # Urls for forums are in the form /<urlpart>/<rest>
  url = db.StringProperty(required=True)
  # What we show as html <title> and as main header on the page
  title = db.StringProperty()
  # a tagline is below title
  tagline = db.StringProperty()
  # stuff to display in left sidebar
  sidebar = db.TextProperty()
  # if true, forum has been disabled. We don't support deletion so that
  # forum can always be re-enabled in the future
  is_disabled = db.BooleanProperty(default=False)
  # just in case, when the forum was created. Not used.
  created_on = db.DateTimeProperty(auto_now_add=True)
  # name of the skin (must be one of SKINS)
  skin = db.StringProperty()
  # Google analytics code
  analytics_code = db.StringProperty()
  # Note: import_secret is obsolete
  import_secret = db.StringProperty()

# A forum is collection of topics
class Topic(db.Model):
  forum = db.Reference(Forum, required=True)
  subject = db.StringProperty(required=True)
  created_on = db.DateTimeProperty(auto_now_add=True)
  # name of person who created the topic. Duplicates Post.user_name
  # of the first post in this topic, for speed
  created_by = db.StringProperty()
  # just in case, not used
  updated_on = db.DateTimeProperty(auto_now=True)
  # True if first Post in this topic is deleted. Updated on deletion/undeletion
  # of the post
  is_deleted = db.BooleanProperty(default=False)
  # ncomments is redundant but is faster than always quering count of Posts
  ncomments = db.IntegerProperty(default=0)

# A topic is a collection of posts
class Post(db.Model):
  topic = db.Reference(Topic, required=True)
  forum = db.Reference(Forum, required=True)
  created_on = db.DateTimeProperty(auto_now_add=True)
  message = db.TextProperty(required=True)
  sha1_digest = db.StringProperty(required=True)
  # admin can delete/undelete posts. If first post in a topic is deleted,
  # that means the topic is deleted as well
  is_deleted = db.BooleanProperty(default=False)
  # ip address from which this post has been made
  user_ip = db.IntegerProperty(required=True)
  user = db.Reference(FofouUser, required=True)
  # user_name, user_email and user_homepage might be different than
  # name/homepage/email fields in user object, since they can be changed in
  # FofouUser
  user_name = db.StringProperty()
  user_email = db.StringProperty()
  user_homepage = db.StringProperty()

SKINS = ["default"]

# cookie code based on http://code.google.com/p/appengine-utitlies/source/browse/trunk/utilities/session.py
FOFOU_COOKIE = "fofou-uid"
COOKIE_EXPIRE_TIME = 60*60*24*120 # valid for 60*60*24*120 seconds => 120 days

def get_user_agent(): return os.environ['HTTP_USER_AGENT']
def get_remote_ip(): return os.environ['REMOTE_ADDR']

def ip2long(ip):
  ip_array = ip.split('.')
  ip_long = int(ip_array[0]) * 16777216 + int(ip_array[1]) * 65536 + int(ip_array[2]) * 256 + int(ip_array[3])
  return ip_long

def long2ip(val):
  slist = []
  for x in range(0,4):
    slist.append(str(int(val >> (24 - (x * 8)) & 0xFF)))
  return ".".join(slist)

def to_unicode(val):
  if isinstance(val, unicode): return val
  try:
    return unicode(val, 'latin-1')
  except:
    pass
  try:
    return unicode(val, 'ascii')
  except:
    pass
  try:
    return unicode(val, 'utf-8')
  except:
    raise

def to_utf8(s):
    s = to_unicode(s)
    return s.encode("utf-8")

def req_get_vals(req, names, strip=True): 
  if strip:
    return [req.get(name).strip() for name in names]
  else:
    return [req.get(name) for name in names]

def get_inbound_cookie():
  c = Cookie.SimpleCookie()
  cstr = os.environ.get('HTTP_COOKIE', '')
  c.load(cstr)
  return c

def new_user_id():
  sid = sha.new(repr(time.time())).hexdigest()
  return sid

def valid_user_cookie(c):
  # cookie should always be a hex-encoded sha1 checksum
  if len(c) != 40:
    return False
  # TODO: check that user with that cookie exists, the way appengine-utilities does
  return True

g_anonUser = None
def anonUser():
  global g_anonUser
  if None == g_anonUser:
    g_anonUser = users.User("dummy@dummy.address.com")
  return g_anonUser

def fake_error(response):
  response.headers['Content-Type'] = 'text/plain'
  response.out.write('There was an error processing your request.')

def valid_forum_url(url):
  if not url:
    return False
  try:
    return url == urllib.quote_plus(url)
  except:
    return False

def sanitize_homepage(s):
    # prevent javascript injection
    if not (s.startswith("http://") or s.startswith("https://")):
        return ""
    # 'http://' is the default value we put, so if unchanged, consider it
    # as not given at all
    if s == "http://": return ""
    return s

# very simplistic check for <txt> being a valid e-mail address
def valid_email(txt):
  # allow empty strings
  if not txt:
    return True
  if '@' not in txt:
    return False
  if '.' not in txt:
    return False
  return True

def forum_from_url(url):
  assert '/' == url[0]
  path = url[1:]
  if '/' in path:
    (forumurl, rest) = path.split("/", 1)
  else:
    forumurl = path
  return Forum.gql("WHERE url = :1", forumurl).get()
      
def forum_root(forum): return "/" + forum.url + "/"

def forum_siteroot_tmpldir_from_url(url):
  assert '/' == url[0]
  path = url[1:]
  if '/' in path:
    (forumurl, rest) = path.split("/", 1)
  else:
    forumurl = path
  forum = Forum.gql("WHERE url = :1", forumurl).get()
  if not forum:
    return (None, None, None)
  siteroot = forum_root(forum)
  skin_name = forum.skin
  if skin_name not in SKINS:
    skin_name = SKINS[0]
  tmpldir = os.path.join("skins", skin_name)
  return (forum, siteroot, tmpldir)

def get_log_in_out(url):
  user = users.get_current_user()
  if user:
    if users.is_current_user_admin():
      return "Welcome admin, %s! <a href=\"%s\">Log out</a>" % (user.nickname(), users.create_logout_url(url))
    else:
      return "Welcome, %s! <a href=\"%s\">Log out</a>" % (user.nickname(), users.create_logout_url(url))
  else:
    return "<a href=\"%s\">Log in or register</a>" % users.create_login_url(url)    

class FofouBase(webapp.RequestHandler):

  _cookie = None
  # returns either a FOFOU_COOKIE sent by the browser or a newly created cookie
  def get_cookie(self):
    if self._cookie != None:
      return self._cookie
    cookies = get_inbound_cookie()
    for cookieName in cookies.keys():
      if FOFOU_COOKIE != cookieName:
        del cookies[cookieName]
    if (FOFOU_COOKIE not in cookies) or not valid_user_cookie(cookies[FOFOU_COOKIE].value):
      cookies[FOFOU_COOKIE] = new_user_id()
      cookies[FOFOU_COOKIE]['path'] = '/'
      cookies[FOFOU_COOKIE]['expires'] = COOKIE_EXPIRE_TIME
    self._cookie = cookies[FOFOU_COOKIE]
    return self._cookie

  _cookie_to_set = None
  # remember cookie so that we can send it when we render a template
  def send_cookie(self):
    if None == self._cookie_to_set:
      self._cookie_to_set = self.get_cookie()

  def get_cookie_val(self):
    c = self.get_cookie()
    return c.value

  def get_fofou_user(self):
    # get user either by google user id or cookie
    user_id = users.get_current_user()
    user = None
    if user_id:
      user = FofouUser.gql("WHERE user = :1", user_id).get()
      #if user: logging.info("Found existing user for by user_id '%s'" % str(user_id))
    else:
      cookie = self.get_cookie_val()
      if cookie:
        user = FofouUser.gql("WHERE cookie = :1", cookie).get()
        #if user:
        #  logging.info("Found existing user for cookie '%s'" % cookie)
        #else:
        #  logging.info("Didn't find user for cookie '%s'" % cookie)
    return user

  def template_out(self, template_name, template_values):
    self.response.headers['Content-Type'] = 'text/html'
    if None != self._cookie_to_set:
      # a hack extract the cookie part from the whole "Set-Cookie: val" header
      c = str(self._cookie_to_set)
      c = c.split(": ", 1)[1]
      self.response.headers["Set-Cookie"] = c
    #path = os.path.join(os.path.dirname(__file__), template_name)
    path = template_name
    #logging.info("tmpl: %s" % path)
    res = template.render(path, template_values)
    self.response.out.write(res)

# responds to GET /manageforums[?forum=<key>&disable=yes&enable=yes]
# and POST /manageforums with values from the form
class ManageForums(FofouBase):

  def post(self):
    if not users.is_current_user_admin():
      return self.redirect("/")

    forum_key = self.request.get('forum_key')
    forum = None
    if forum_key:
      forum = db.get(db.Key(forum_key))
      if not forum:
        # invalid key - should not happen so go to top-level
        return self.redirect("/")

    vals = ['url','title', 'tagline', 'sidebar', 'disable', 'enable', 'analyticscode']
    (url, title, tagline, sidebar, disable, enable, analytics_code) = req_get_vals(self.request, vals)

    errmsg = None
    if not valid_forum_url(url):
      errmsg = "Url contains illegal characters"
    if not forum:
      forum_exists = Forum.gql("WHERE url = :1", url).get()
      if forum_exists:
        errmsg = "Forum with this url already exists"

    if errmsg:
      tvals = {
        'urlclass' : "error",
        'hosturl' : self.request.host_url,
        'prevurl' : url,
        'prevtitle' : title,
        'prevtagline' : tagline,
        'prevsidebar' : sidebar,
        'prevanalyticscode' : analytics_code,
        'forum_key' : forum_key,
        'errmsg' : errmsg
      }
      return self.render_rest(tvals)

    title_or_url = title or url
    if forum:
      # update existing forum
      forum.url = url
      forum.title = title
      forum.tagline = tagline
      forum.sidebar = sidebar
      forum.analytics_code = analytics_code
      forum.put()
      msg = "Forum '%s' has been updated." % title_or_url
    else:
      # create a new forum
      forum = Forum(url=url, title=title, tagline=tagline, sidebar=sidebar, analytics_code = analytics_code)
      forum.put()
      msg = "Forum '%s' has been created." % title_or_url
    url = "/manageforums?msg=%s" % urllib.quote(to_utf8(msg))
    return self.redirect(url)

  def get(self):
    if not users.is_current_user_admin():
      return self.redirect("/")

    # if there is 'forum_key' argument, this is editing an existing forum.
    forum = None
    forum_key = self.request.get('forum_key')
    if forum_key:
      forum = db.get(db.Key(forum_key))
      if not forum:
        # invalid forum key - should not happen, return to top level
        return self.redirect("/")

    tvals = {
      'hosturl' : self.request.host_url,
      'forum' : forum
    }
    if forum:
      forum.title_non_empty = forum.title or "Title."
      forum.sidebar_non_empty = forum.sidebar or "Sidebar." 
      disable = self.request.get('disable')
      enable = self.request.get('enable')
      if disable or enable:
        title_or_url = forum.title or forum.url
        if disable:
          forum.is_disabled = True
          forum.put()
          msg = "Forum %s has been disabled." % title_or_url
        else:
          forum.is_disabled = False
          forum.put()
          msg = "Forum %s has been enabled." % title_or_url
        return self.redirect("/manageforums?msg=%s" % urllib.quote(to_utf8(msg)))
    self.render_rest(tvals, forum)

  def render_rest(self, tvals, forum=None):
    user = users.get_current_user()
    forumsq = db.GqlQuery("SELECT * FROM Forum")
    forums = []
    for f in forumsq:
      f.title_or_url = f.title or f.url
      edit_url = "/manageforums?forum_key=" + str(f.key())
      if f.is_disabled:
        f.enable_disable_txt = "enable"
        f.enable_disable_url = edit_url + "&enable=yes"
      else:
        f.enable_disable_txt = "disable"
        f.enable_disable_url = edit_url + "&disable=yes"      
      if forum and f.key() == forum.key():
        # editing existing forum
        f.no_edit_link = True
        tvals['prevurl'] = f.url
        tvals['prevtitle'] = f.title
        tvals['prevtagline'] = f.tagline
        tvals['prevsidebar'] = f.sidebar
        tvals['prevanalyticscode'] = f.analytics_code
        tvals['forum_key'] = str(f.key())
      forums.append(f)
    tvals['msg'] = self.request.get('msg')
    tvals['user'] = user
    tvals['forums'] = forums
    if forum and not forum.tagline:
      forum.tagline = "Tagline."
    self.template_out("manage_forums.html", tvals)

# responds to /, shows list of available forums or redirects to
# forum management page if user is admin
class ForumList(FofouBase):
  def get(self):
    if users.is_current_user_admin():
      return self.redirect("/manageforums")
    MAX_FORUMS = 256 # if you need more, tough
    forums = db.GqlQuery("SELECT * FROM Forum").fetch(MAX_FORUMS)
    for f in forums:
        f.title_or_url = f.title or f.url
    tvals = {
      'forums' : forums,
      'isadmin' : users.is_current_user_admin(),
      'log_in_out' : get_log_in_out("/")
    }
    self.template_out("forum_list.html", tvals)

# responds to GET /postdel?<post_id> and /postundel?<post_id>
class PostDelUndel(webapp.RequestHandler):
  def get(self):
    (forum, siteroot, tmpldir) = forum_siteroot_tmpldir_from_url(self.request.path_info)
    if not forum or forum.is_disabled:
      return self.redirect("/")
    is_moderator = users.is_current_user_admin()
    if not is_moderator or forum.is_disabled:
      return self.redirect(siteroot)
    post_id = self.request.query_string
    #logging.info("PostDelUndel: post_id='%s'" % post_id)
    post = db.get(db.Key.from_path('Post', int(post_id)))
    if not post:
      logging.info("No post with post_id='%s'" % post_id)
      return self.redirect(siteroot)
    if post.forum.key() != forum.key():
      loggin.info("post.forum.key().id() ('%s') != fourm.key().id() ('%s')" % (str(post.forum.key().id()), str(forum.key().id())))
      return self.redirect(siteroot)
    path = self.request.path
    if path.endswith("/postdel"):
      if not post.is_deleted:
        post.is_deleted = True
        post.put()
        memcache.delete(rss_memcache_key(forum))
      else:
        logging.info("Post '%s' is already deleted" % post_id)
    elif path.endswith("/postundel"):
      if post.is_deleted:
        post.is_deleted = False
        post.put()
        memcache.delete(rss_memcache_key(forum))
      else:
        logging.info("Trying to undelete post '%s' that is not deleted" % post_id)
    else:
      logging.info("'%s' is not a valid path" % path)

    topic = post.topic
    # deleting/undeleting first post also means deleting/undeleting the whole topic
    first_post = Post.gql("WHERE forum=:1 AND topic = :2 ORDER BY created_on", forum, topic).get()
    if first_post.key() == post.key():
      if path.endswith("/postdel"):
        topic.is_deleted = True
      else:
        topic.is_deleted = False
      topic.put()

    # redirect to topic owning this post
    topic_url = siteroot + "topic?id=" + str(topic.key().id())
    self.redirect(topic_url)
    
# responds to /<forumurl>/[?from=<from>]
# shows a list of topics, potentially starting from topic N
class TopicList(FofouBase):

  def get_topics(self, forum, is_moderator, max_topics, cursor):
    # note: building query manually beccause gql() don't work with cursor
    # see: http://code.google.com/p/googleappengine/issues/detail?id=2757
    q = Topic.all()
    q.filter("forum =", forum)
    if not is_moderator:
        q.filter("is_deleted =", False)
    q.order("-created_on")
    if not cursor is None:
      q.with_cursor(cursor)
    topics = q.fetch(max_topics)
    new_cursor = q.cursor()
    if len(topics) < max_topics:
        new_cursor = None
    return (new_cursor, topics)

  def get(self):
    (forum, siteroot, tmpldir) = forum_siteroot_tmpldir_from_url(self.request.path_info)
    if not forum or forum.is_disabled:
      return self.redirect("/")
    cursor = self.request.get("from") or None
    is_moderator = users.is_current_user_admin()
    MAX_TOPICS = 75
    (new_cursor, topics) = self.get_topics(forum, is_moderator, MAX_TOPICS, cursor)
    forum.title_or_url = forum.title or forum.url
    tvals = {
      'siteroot' : siteroot,
      'siteurl' : self.request.url,
      'forum' : forum,
      'topics' : topics,
      'analytics_code' : forum.analytics_code or "",
      'new_from' : new_cursor,
      'log_in_out' : get_log_in_out(siteroot)
    }
    tmpl = os.path.join(tmpldir, "topic_list.html")
    self.template_out(tmpl, tvals)

# responds to /<forumurl>/topic?id=<id>
class TopicForm(FofouBase):

  def get(self):
    (forum, siteroot, tmpldir) = forum_siteroot_tmpldir_from_url(self.request.path_info)
    if not forum or forum.is_disabled:
      return self.redirect("/")
    forum.title_or_url = forum.title or forum.url

    topic_id = self.request.get('id')
    if not topic_id:
      return self.redirect(siteroot)

    topic = db.get(db.Key.from_path('Topic', int(topic_id)))
    if not topic:
      return self.redirect(siteroot)

    is_moderator = users.is_current_user_admin()
    if topic.is_deleted and not is_moderator:
      return self.redirect(siteroot)

    is_archived = False
    # Note: auto-archiving disabled
    #now = datetime.datetime.now()
    #week = datetime.timedelta(days=7)
    #week = datetime.timedelta(seconds=7)
    #if now > topic.created_on + week:
    #  is_archived = True

    # 200 is more than generous
    MAX_POSTS = 200
    if is_moderator:
      posts = Post.gql("WHERE forum = :1 AND topic = :2 ORDER BY created_on", forum, topic).fetch(MAX_POSTS)
    else:
      posts = Post.gql("WHERE forum = :1 AND topic = :2 AND is_deleted = False ORDER BY created_on", forum, topic).fetch(MAX_POSTS)

    if is_moderator:
        for p in posts:
            p.user_ip_str = long2ip(p.user_ip)
            if p.user_homepage:
                p.user_homepage = sanitize_homepage(p.user_homepage)
    tvals = {
      'siteroot' : siteroot,
      'forum' : forum,
      'analytics_code' : forum.analytics_code or "",
      'topic' : topic,
      'is_moderator' : is_moderator,
      'is_archived' : is_archived,
      'posts' : posts,
      'log_in_out' : get_log_in_out(self.request.url),
    }
    tmpl = os.path.join(tmpldir, "topic.html")
    self.template_out(tmpl, tvals)

# responds to /<forumurl>/rss, returns an RSS feed of recent topics
# (taking into account only the first post in a topic - that's what
# joelonsoftware forum rss feed does)
class RssFeed(webapp.RequestHandler):

  def get(self):
    (forum, siteroot, tmpldir) = forum_siteroot_tmpldir_from_url(self.request.path_info)
    if not forum or forum.is_disabled:
      return self.error(HTTP_NOT_FOUND)

    cached_feed = memcache.get(rss_memcache_key(forum))
    if cached_feed is not None:
      self.response.headers['Content-Type'] = 'text/xml'
      self.response.out.write(cached_feed)
      return
      
    feed = feedgenerator.Atom1Feed(
      title = forum.title or forum.url,
      link = my_hostname() + siteroot + "rss",
      description = forum.tagline)
  
    topics = Topic.gql("WHERE forum = :1 AND is_deleted = False ORDER BY created_on DESC", forum).fetch(25)
    for topic in topics:
      title = topic.subject
      link = my_hostname() + siteroot + "topic?id=" + str(topic.key().id())
      first_post = Post.gql("WHERE topic = :1 ORDER BY created_on", topic).get()
      msg = first_post.message
      # TODO: a hack: using a full template to format message body.
      # There must be a way to do it using straight django APIs
      name = topic.created_by
      if name:
        t = Template("<strong>{{ name }}</strong>: {{ msg|striptags|escape|urlize|linebreaksbr }}")
      else:
        t = Template("{{ msg|striptags|escape|urlize|linebreaksbr }}")
      c = Context({"msg": msg, "name" : name})
      description = t.render(c)
      pubdate = topic.created_on
      feed.add_item(title=title, link=link, description=description, pubdate=pubdate)
    feedtxt = feed.writeString('utf-8')
    self.response.headers['Content-Type'] = 'text/xml'
    self.response.out.write(feedtxt)
    memcache.add(rss_memcache_key(forum), feedtxt)

# responds to /<forumurl>/rssall, returns an RSS feed of all recent posts
# This is good for forum admins/moderators who want to monitor all posts
class RssAllFeed(webapp.RequestHandler):

  def get(self):
    (forum, siteroot, tmpldir) = forum_siteroot_tmpldir_from_url(self.request.path_info)
    if not forum or forum.is_disabled:
      return self.error(HTTP_NOT_FOUND)

    feed = feedgenerator.Atom1Feed(
      title = forum.title or forum.url,
      link = my_hostname() + siteroot + "rssall",
      description = forum.tagline)
  
    posts = Post.gql("WHERE forum = :1 AND is_deleted = False ORDER BY created_on DESC", forum).fetch(25)
    for post in posts:
      topic = post.topic
      title = topic.subject
      link = my_hostname() + siteroot + "topic?id=" + str(topic.key().id())
      msg = post.message
      # TODO: a hack: using a full template to format message body.
      # There must be a way to do it using straight django APIs
      name = post.user_name
      if name:
        t = Template("<strong>{{ name }}</strong>: {{ msg|striptags|escape|urlize|linebreaksbr }}")
      else:
        t = Template("{{ msg|striptags|escape|urlize|linebreaksbr }}")
      c = Context({"msg": msg, "name" : name})
      description = t.render(c)
      pubdate = post.created_on
      feed.add_item(title=title, link=link, description=description, pubdate=pubdate)
    feedtxt = feed.writeString('utf-8')
    self.response.headers['Content-Type'] = 'text/xml'
    self.response.out.write(feedtxt)

# responds to /<forumurl>/email[?post_id=<post_id>]
class EmailForm(FofouBase):

  def get(self):
    (forum, siteroot, tmpldir) = forum_siteroot_tmpldir_from_url(self.request.path_info)
    if not forum or forum.is_disabled:
      return self.redirect("/")
    (num1, num2) = (random.randint(1,9), random.randint(1,9))
    post_id = self.request.get("post_id")
    if not post_id: return self.redirect(siteroot)
    post = db.get(db.Key.from_path('Post', int(post_id)))
    if not post: return self.redirect(siteroot)
    to_name = post.user_name or post.user_homepage
    subject = "Re: " + (forum.title or forum.url) + " - " + post.topic.subject
    forum.title_or_url = forum.title or forum.url
    tvals = {
      'siteroot' : siteroot,
      'forum' : forum,
      'num1' : num1,
      'num2' : num2,
      'num3' : int(num1) + int(num2),
      'post_id' : post_id,
      'to' : to_name,
      'subject' : subject,
      'log_in_out' : get_log_in_out(siteroot + "post")
    }
    tmpl = os.path.join(tmpldir, "email.html")
    self.template_out(tmpl, tvals)

  def post(self):
    (forum, siteroot, tmpldir) = forum_siteroot_tmpldir_from_url(self.request.path_info)
    if not forum or forum.is_disabled:
      return self.redirect("/")
    if self.request.get('Cancel'): self.redirect(siteroot)
    post_id = self.request.get("post_id")
    #logging.info("post_id = %s" % str(post_id))
    if not post_id: return self.redirect(siteroot)
    post = db.get(db.Key.from_path('Post', int(post_id)))
    if not post: return self.redirect(siteroot)
    topic = post.topic
    tvals = {
      'siteroot' : siteroot,
      'forum' : forum,
      'topic' : topic,
      'log_in_out' : get_log_in_out(siteroot + "post")
    }    
    tmpl = os.path.join(tmpldir, "email_sent.html")
    self.template_out(tmpl, tvals)

# responds to /<forumurl>/post[?id=<topic_id>]
class PostForm(FofouBase):

  def get(self):
    (forum, siteroot, tmpldir) = forum_siteroot_tmpldir_from_url(self.request.path_info)
    if not forum or forum.is_disabled:
      return self.redirect("/")

    ip = get_remote_ip()
    if ip in BANNED_IPS:
      return fake_error(self.response)

    self.send_cookie()

    rememberChecked = ""
    prevUrl = "http://"
    prevEmail = ""
    prevName = ""
    user = self.get_fofou_user()
    if user and user.remember_me:
      rememberChecked = "checked"
      prevUrl = user.homepage
      if not prevUrl:
        prevUrl = "http://"
      prevName = user.name
      prevEmail = user.email
    (num1, num2) = (random.randint(1,9), random.randint(1,9))
    forum.title_or_url = forum.title or forum.url
    tvals = {
      'siteroot' : siteroot,
      'forum' : forum,
      'num1' : num1,
      'num2' : num2,
      'num3' : int(num1) + int(num2),
      'rememberChecked' : rememberChecked,
      'prevUrl' : prevUrl,
      'prevEmail' : prevEmail,
      'prevName' : prevName,
      'log_in_out' : get_log_in_out(self.request.url)
    }
    topic_id = self.request.get('id')
    if topic_id:
      topic = db.get(db.Key.from_path('Topic', int(topic_id)))
      if not topic: return self.redirect(siteroot)
      tvals['prevTopicId'] = topic_id
      tvals['prevSubject'] = topic.subject
    tmpl = os.path.join(tmpldir, "post.html")
    self.template_out(tmpl, tvals)

  def post(self):
    (forum, siteroot, tmpldir) = forum_siteroot_tmpldir_from_url(self.request.path_info)
    if not forum or forum.is_disabled:
      return self.redirect("/")
    if self.request.get('Cancel'): 
      return self.redirect(siteroot)

    ip = get_remote_ip()
    if ip in BANNED_IPS:
      return self.redirect(siteroot)

    self.send_cookie()

    vals = ['TopicId', 'num1', 'num2', 'Captcha', 'Subject', 'Message', 'Remember', 'Email', 'Name', 'Url']
    (topic_id, num1, num2, captcha, subject, message, remember_me, email, name, homepage) = req_get_vals(self.request, vals)
    message = to_unicode(message)

    remember_me = True
    if remember_me == "": remember_me = False
    rememberChecked = ""
    if remember_me: rememberChecked = "checked"

    validCaptcha = True
    try:
      captcha = int(captcha)
      num1 = int(num1)
      num2 = int(num2)
    except ValueError:
      validCaptcha = False

    homepage = sanitize_homepage(homepage)
    tvals = {
      'siteroot' : siteroot,
      'forum' : forum,
      'num1' : num1,
      'num2' : num2,
      'num3' : int(num1) + int(num2),
      "prevCaptcha" : captcha,
      "prevSubject" : subject,
      "prevMessage" : message,
      "rememberChecked" : rememberChecked,
      "prevEmail" : email,
      "prevUrl" : homepage,
      "prevName" : name,
      "prevTopicId" : topic_id,
      "log_in_out" : get_log_in_out(siteroot + "post")
    }
    
    # validate captcha and other values
    errclass = None
    if not validCaptcha or (captcha != (num1 + num2)): errclass = 'captcha_class'
    if not message: errclass = "message_class"
    if not name: errclass = "name_class"
    if not valid_email(email): errclass = "email_class"
    # first post must have subject
    if not topic_id and not subject: errclass = "subject_class"

    # sha.new() doesn't accept Unicode strings, so convert to utf8 first
    message_utf8 = message.encode('UTF-8')
    s = sha.new(message_utf8)
    sha1_digest = s.hexdigest()

    duppost = Post.gql("WHERE sha1_digest = :1", sha1_digest).get()
    if duppost: errclass = "message_class"

    if errclass:
      tvals[errclass] = "error"
      tmpl = os.path.join(tmpldir, "post.html")
      return self.template_out(tmpl, tvals)

    # get user either by google user id or cookie. Create user objects if don't
    # already exist
    existing_user = False
    user_id = users.get_current_user()
    if user_id:
      user = FofouUser.gql("WHERE user = :1", user_id).get()
      if not user:
        #logging.info("Creating new user for '%s'" % str(user_id))
        user = FofouUser(user=user_id, remember_me = remember_me, email=email, name=name, homepage=homepage)
        user.put()
      else:
        existing_user = True
        #logging.info("Found existing user for '%s'" % str(user_id))
    else:
      cookie = self.get_cookie_val()
      user = FofouUser.gql("WHERE cookie = :1", cookie).get()
      if not user:
        #logging.info("Creating new user for cookie '%s'" % cookie)
        user = FofouUser(cookie=cookie, remember_me = remember_me, email=email, name=name, homepage=homepage)
        user.put()
      else:
        existing_user = True
        #logging.info("Found existing user for cookie '%s'" % cookie)

    if existing_user:
      need_update = False
      if user.remember_me != remember_me:
        user.remember_me = remember_me
        need_update = True
      if user.email != email:
        user.email = email
        need_update = True
      if user.name != name:
        user.name = name
        need_update = True
      if user.homepage != homepage:
        user.homepage = homepage
        need_update = True
      if need_update:
        #logging.info("User needed an update")
        user.put()

    if not topic_id:
      topic = Topic(forum=forum, subject=subject, created_by=name)
      topic.put()
    else:
      topic = db.get(db.Key.from_path('Topic', int(topic_id)))
      #assert forum.key() == topic.forum.key()
      topic.ncomments += 1
      topic.put()

    user_ip = ip2long(get_remote_ip())
    p = Post(topic=topic, forum=forum, user=user, user_ip=user_ip, message=message, sha1_digest=sha1_digest, user_name = name, user_email = email, user_homepage = homepage)
    p.put()
    memcache.delete(rss_memcache_key(forum))
    if topic_id:
      self.redirect(siteroot + "topic?id=" + str(topic_id))
    else:
      self.redirect(siteroot)

def main():
  application = webapp.WSGIApplication(
     [  ('/', ForumList),
        ('/manageforums', ManageForums),
        ('/[^/]+/postdel', PostDelUndel),
        ('/[^/]+/postundel', PostDelUndel),
        ('/[^/]+/post', PostForm),
        ('/[^/]+/topic', TopicForm),
        ('/[^/]+/email', EmailForm),
        ('/[^/]+/rss', RssFeed),
        ('/[^/]+/rssall', RssAllFeed),
        ('/[^/]+/?', TopicList)],
     debug=True)
  wsgiref.handlers.CGIHandler().run(application)

if __name__ == "__main__":
  main()
