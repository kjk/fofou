import os, string, Cookie, sha, time
import wsgiref.handlers
from google.appengine.ext import db
from google.appengine.api import users
from google.appengine.ext import webapp
from google.appengine.ext.webapp import template

# Structure of urls:
# /<forum_url>/<rest>
#
# <rest> is:
# /
#    index, lists recent topics
# /post[?topic=<id>]
#    form for creating a new post. if "topic" is present, it's a new post in
#    existing topic, otherwise a new topic post
# /topic/<id>?posts=<n>
#    shows posts in a given topic, 'posts' is ignored (just a trick to re-use
#    browser's history to see if the topic has posts that user didn't see yet
# /rss
#    rss feed for posts

# cookie code based on http://code.google.com/p/appengine-utitlies/source/browse/trunk/utilities/session.py
# TODO: cookie validation

COOKIE_NAME = "fofou-uid"
COOKIE_PATH = "/"
COOKIE_EXPIRE_TIME = 60*60*24*120 # valid for 60*60*24*120 seconds => 120 days
HTTP_COOKIE_HDR = "HTTP_COOKIE"

def get_user_agent(): return os.environ['HTTP_USER_AGENT']
def get_remote_ip(): return os.environ['REMOTE_ADDR']

def get_inbound_cookie():
  c = Cookie.SimpleCookie()
  cstr = os.environ.get(HTTP_COOKIE_HDR, '')
  c.load(cstr)
  return c

def new_user_id():
  sid = sha.new(repr(time.time())).hexdigest()
  return sid

def get_user_cookie()::
  c = get_inbound_cookie()
  if COOKIE_NAME not in c:
    c[COOKIE_NAME] = new_user_id()
    c[COOKIE_NAME]['path'] = COOKIE_PATH
    c[COOKIE_NAME]['expires'] = COOKIE_EXPIRE_TIME
  # TODO: maybe should validate cookie if exists, the way appengine-utilities does
  return c

class Forum(db.Model):
  # Urls for forums are in the form /<urlpart>/<rest>
  url = db.StringProperty(required=True)
  # What we show as html <title>
  title = db.StringProperty()
  tagline = db.StringProperty()
  sideline = db.StringProperty()

class Topic(db.Model):
  subject = db.StringProperty(required=True)
  created = db.DateTimeProperty(auto_now_add=True)
  updated = db.DateTimeProperty(auto_now=True)
  archived = db.BooleanProperty(default=False)

class AnonUser(db.Model):
  uidcookie = db.StringProperty()
  ipaddr = db.StringProperty()

class Post(db.Model):
  created = db.DateTimeProperty(auto_now_add=True)
  body = db.Text(required=True)
  topic = db.Reference(Topic)
  user = db.Reference(AnonUser)

class CreateForum(webapp.RequestHandler):
  def cant_create(self):
    self.response.headers['Content-Type'] = 'text/html'
    tname = "cant_create_forum.html"
    tvals = {}
    path = os.path.join(os.path.dirname(__file__), tname)
    self.response.out.write(template.render(path, tvals))

  def valid_url(self, url):
    return url.isalpha()

  def post(self):
    if not users.is_current_user_admin():
      self.cant_create()
      return
    self.response.headers['Content-Type'] = 'text/html'
    url = self.request.get('url')
    title = self.request.get('title')
    tagline = self.request.get('tagline')
    sidebar = self.request.get('sidebar')
    if not self.valid_url(url):
      tname = 'create_forum_invalid.html'
      errmsg = "Url '%s' is not valid. Can only contain letters." % url
      tvals = {
        'errmsg' : errmsg
      }
      path = os.path.join(os.path.dirname(__file__), tname)
      self.response.out.write(template.render(path, tvals))
      return
    forum = Forum(url=url)
    forum.title = title
    forum.tagline = tagline
    forum.sidebar = sidebar
    forum.put()

    tname = 'forum_created.html'
    forumname = url
    forumurl = "/" + url + "/"
    tvals = {
      'forumname' : forumname,
      'forumurl' : forumurl,
      'forumlisturl' : "/createforum"
    }
    path = os.path.join(os.path.dirname(__file__), tname)
    self.response.out.write(template.render(path, tvals))
    return

  def get(self):
    if users.is_current_user_admin():
      self.user = users.get_current_user()
      tname = "no_forums_admin.html"
      forumsq = db.GqlQuery("SELECT * FROM Forum")
      forums = []
      for f in forumsq:
        forums.append(f)
      tvals = {}
      tvals['nickname'] = self.user.nickname()
      tvals['forums'] = forums
      path = os.path.join(os.path.dirname(__file__), tname)
      self.response.out.write(template.render(path, tvals))
    else:
      self.cant_create()

class Dispatcher(webapp.RequestHandler):
  def no_forums(self):
    self.response.headers['Content-Type'] = 'text/html'
    tname = None
    tvals = {}
    if not self.user:
      tname = "no_forums_not_logged_in.html"
      tvals['loginurl'] = users.create_login_url(self.request.uri)
    elif users.is_current_user_admin():
      tname = "no_forums_admin.html"
      forums = self.forums
      #forums = None
      #if len(self.forums) > 0:
      #  forums = self.forums
      tvals['nickname'] = self.user.nickname()
      tvals['forums'] = forums
    else:
      tname = "no_forums_not_admin.html"
      tvals['logouturl'] = users.create_logout_url(self.request.uri)
    path = os.path.join(os.path.dirname(__file__), tname)
    self.response.out.write(template.render(path, tvals))

  def get_forum(self, url):
    forum = Forum.gql("WHERE url = :1", url)
    return forum.get()

  def do_forum(self, forum):
    self.response.headers['Content-Type'] = 'text/html'
    tname = "index.html"
    tvals = {}
    title = forum.title
    if 0 == len(title):
      title = forum.url
    tvals['title'] = title
    path = os.path.join(os.path.dirname(__file__), tname)
    self.response.out.write(template.render(path, tvals))

  def get(self):
    # TODO: do I need to cache it or is it cached by the system?
    self.user = users.get_current_user()
    req = self.request
    path = req.path_info[1:]
    forumurl = None
    pathrest = None
    if '/' in path:
      (forumurl, pathrest) = path.split("/", 1)
    else:
      forumurl = path
      pathrest = ""

    forum = self.get_forum(forumurl)
    if forum:
      return self.do_forum(forum)

    forumsq = db.GqlQuery("SELECT * FROM Forum")
    self.forums = []
    for f in forumsq:
      self.forums.append(f)
    forumcount = len(self.forums)
    if 0 == forumcount:
      return self.no_forums()
    isadmin = users.is_current_user_admin()
    for f in self.forums:
      if 0 == len(f.title):
        f.title = f.url
    self.response.headers['Content-Type'] = 'text/html'
    tname = "forum_list.html"
    tvals = {
      'isadmin' : isadmin,
      'createforumurl' : "/createforum",
      'forums' : self.forums,
      'path' : path,
      'forumurl' : forumurl,
      'pathrest' : pathrest
    }
    path = os.path.join(os.path.dirname(__file__), tname)
    self.response.out.write(template.render(path, tvals))

def main():
  application = webapp.WSGIApplication( 
     [ ('/createforum', CreateForum),
       ('.*', Dispatcher)],
     debug=True)
  wsgiref.handlers.CGIHandler().run(application)

if __name__ == "__main__":
  main()
