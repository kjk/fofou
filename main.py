import os, string, Cookie, sha, time, random
import wsgiref.handlers
from google.appengine.ext import db
from google.appengine.api import users
from google.appengine.ext import webapp
from google.appengine.ext.webapp import template
import logging

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

def get_user_cookie():
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
  sidebar = db.StringProperty()

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
  body = db.TextProperty(required=True)
  topic = db.Reference(Topic)
  user = db.Reference(AnonUser)

class ForumsManage(webapp.RequestHandler):
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
      self.response.headers['Content-Type'] = 'text/html'
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
      'forums_manage_url' : "/forumsmanage"
    }
    self.response.headers['Content-Type'] = 'text/html'
    path = os.path.join(os.path.dirname(__file__), tname)
    self.response.out.write(template.render(path, tvals))
    return

  def get(self):
    if users.is_current_user_admin():
      user = users.get_current_user()
      tname = "no_forums_admin.html"
      forumsq = db.GqlQuery("SELECT * FROM Forum")
      forums = []
      for f in forumsq:
        forums.append(f)
      tvals = {
        'nickname' : user.nickname(),
        'forums' : forums
      }
      self.response.headers['Content-Type'] = 'text/html'
      path = os.path.join(os.path.dirname(__file__), tname)
      self.response.out.write(template.render(path, tvals))
    else:
      self.cant_create()

def forum_from_url(url):
  assert '/' == url[0]
  path = url[1:]
  if '/' in path:
    (forumurl, rest) = path.split("/", 1)
  else:
    forumurl = path
  forum = Forum.gql("WHERE url = :1", forumurl)
  return forum.get()

class IndexForm(webapp.RequestHandler):
  def get(self):
    pass

  # TODO: merge no_fourms_not_logged_in.html, no_forums_admin.html and
  # no_forums_not_admin.html into forum_list.html, to simplify
  def no_forums(self):
    user = users.get_current_user()
    tname = None
    tvals = {}
    if not user:
      tname = "no_forums_not_logged_in.html"
      tvals['loginurl'] = users.create_login_url(self.request.uri)
    elif users.is_current_user_admin():
      tname = "no_forums_admin.html"
      tvals['nickname'] = user.nickname()
    else:
      tname = "no_forums_not_admin.html"
      tvals['logouturl'] = users.create_logout_url(self.request.uri)
    self.response.headers['Content-Type'] = 'text/html'
    path = os.path.join(os.path.dirname(__file__), tname)
    self.response.out.write(template.render(path, tvals))

  def forum_list(self):
    forumsq = db.GqlQuery("SELECT * FROM Forum")
    if 0 == forumsq.count():
      return self.no_forums()
    forums = []
    for f in forumsq:
      # if title is missing, make it same as url
      f.title = f.title or f.url
      forums.append(f)
    isadmin = users.is_current_user_admin()
    tvals = {
      'isadmin' : isadmin,
      'forums' : forums,
    }
    self.response.headers['Content-Type'] = 'text/html'
    path = os.path.join(os.path.dirname(__file__), "forum_list.html")
    self.response.out.write(template.render(path, tvals))

  def forum_index(self, forum):
    assert forum
    tvals = {
      'title' : forum.title or forum.url,
      'posturl' : "/" + forum.url + "/post",
      'archiveurl' : "/" + forum.url + "/archive"
    }
    self.response.headers['Content-Type'] = 'text/html'
    path = os.path.join(os.path.dirname(__file__), "index.html")
    self.response.out.write(template.render(path, tvals))

# responds to /<forumurl>/post
class PostForm(IndexForm):

  def get(self):
    forum = forum_from_url(self.request.path_info)
    if not forum:
      return self.forum_list()

    topicid = self.request.get('topic')

    (num1, num2) = (random.randint(1,9), random.randint(1,9))
    tvals = {
      'title' : forum.title or forum.url,
      'siteroot' : "/" + forum.url,
      'sidebar' : forum.sidebar or "",
      'tagline' : forum.tagline or "",
      'num1' : num1,
      'num2' : num2,
      'num3' : num1+num2,
      'url_val' : "http://"
    }
    self.response.headers['Content-Type'] = 'text/html'
    path = os.path.join(os.path.dirname(__file__), "post.html")
    self.response.out.write(template.render(path, tvals))

  def post(self):
    forum = forum_from_url(self.request.path_info)
    if not forum:
      return self.forum_list()
    # TODO: read form values and act apropriately
    if self.request.get('Cancel'):
      # TODO: should redirect instead?
      return self.forum_index(forum)

    self.forum_index(forum)

class Dispatcher(IndexForm):

  def get(self):
    forum = forum_from_url(self.request.path_info)
    if not forum:
      return self.forum_list()

    self.forum_index(forum)

def main():
  application = webapp.WSGIApplication( 
     [ ('/forumsmanage', ForumsManage),
       ('/[^/]*/post', PostForm),
       ('.*', Dispatcher)],
     debug=True)
  wsgiref.handlers.CGIHandler().run(application)

if __name__ == "__main__":
  main()
