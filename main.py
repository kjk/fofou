import os, string, Cookie, sha, time, random
import wsgiref.handlers
from google.appengine.ext import db
from google.appengine.api import users
from google.appengine.ext import webapp
from google.appengine.ext.webapp import template
import logging

# Structure of urls:
#
# Top-level urls
#
# / - list of all forums
# /forumsmanage[?forum=<key> - edit/create/disable forums
#
# Per-forum urls
#
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
  sidebar = db.TextProperty()
  disabled = db.BooleanProperty(default=False)

class Topic(db.Model):
  forum = db.Reference(Forum, required=True)
  subject = db.StringProperty(required=True)
  created = db.DateTimeProperty(auto_now_add=True)
  updated = db.DateTimeProperty(auto_now=True)
  archived = db.BooleanProperty(default=False)
  # ncomments is redundant for perf
  ncomments = db.IntegerProperty(default=0)

class Post(db.Model):
  created = db.DateTimeProperty(auto_now_add=True)
  body = db.TextProperty(required=True)
  topic = db.Reference(Topic, required=True)
  # ip address from which this post has been made
  user_ip = db.StringProperty(required=True)
  # a cookie for this user (if anonymous)
  user_cookie = db.StringProperty()
  # user id for logged in users
  user_id = db.UserProperty()

def template_out(response, template_name, template_values):
  response.headers['Content-Type'] = 'text/html'
  path = os.path.join(os.path.dirname(__file__), template_name)
  response.out.write(template.render(path, template_values))
  
# responds to /forumsmanage[?forum=<key>&disable=yes]
class ForumsManage(webapp.RequestHandler):

  def valid_url(self, url):
    return url.isalpha()

  def post(self):
    if not users.is_current_user_admin():
      return self.redirect("/")

    url = self.request.get('url')
    title = self.request.get('title')
    tagline = self.request.get('tagline')
    sidebar = self.request.get('sidebar')
    # TODO: disabled or not
    if not self.valid_url(url):
      # TODO: error case should re-use the same form, just indicate an error
      # directly in the form
      errmsg = "Url '%s' is not valid. Can only contain letters." % url
      tvals = {
        'errmsg' : errmsg
      }
      template_out(self.response,  "create_forum_invalid.html", tvals)
      return

    forum = Forum(url=url, title=title, tagline=tagline, sidebar=sidebar)
    forum.put()

    forumname = url
    forumurl = "/" + url + "/"
    tvals = {
      'forumname' : forumname,
      'forumurl' : forumurl,
      'forums_manage_url' : "/forumsmanage"
    }
    # TODO: redirect this to itself with a message that will flash on the
    # screen (using Javascript or just static div)
    template_out(self.response,  "forum_created.html", tvals)

  def get(self):
    if not users.is_current_user_admin():
      return self.redirect("/")

    # if there is 'forum_key' argument, this is editing an existing
    # forum.
    forum = None
    forum_key = self.request.get('forum')
    if forum_key:
      forum = db.get(db.Key(forum_key))
    if forum:
      disable = self.request.get('disable')
      enable = self.request.get('enable')
      if disable:
        # TODO: disable enabled forum
        pass
      elif enable:
        # TODO: enable disabled forum
        pass
      # TODO: populate form with url/title etc. from forum

    user = users.get_current_user()
    forumsq = db.GqlQuery("SELECT * FROM Forum")
    forums = []
    for f in forumsq:
      if not f.title:
        f.title = f.url
      f.edit_url = "/forumsmanage?forum=" + str(f.key())
      if f.disabled:
        f.enable_disable_txt = "enable"
        f.enable_disable_url = f.edit_url + "&enable=yes"
      else:
        f.enable_disable_txt = "disable"
        f.enable_disable_url = f.edit_url + "&disable=yes"      
      forums.append(f)

    tvals = {
      'user' : user,
      'forums' : forums,
    }
    template_out(self.response,  "no_forums_admin.html", tvals)

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
    template_out(self.response, tname, tvals)

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
    template_out(self.response,  "forum_list.html", tvals)

  def forum_index(self, forum):
    assert forum
    # TODO: filter by forum
    topics = [massage_topic(t) for t in db.GqlQuery("SELECT * FROM Topic")]
    MAX_TOPICS = 25
    topics = topics[:MAX_TOPICS]
    tvals = {
      'title' : forum.title or forum.url,
      'posturl' : "/" + forum.url + "/post",
      'archiveurl' : "/" + forum.url + "/archive",
      'siteroot' : "/" + forum.url,
      'topics' : topics
    }
    template_out(self.response,  "index.html", tvals)

def massage_topic(topic):
  # TODO: should update topic with message count when constructing a message
  # to avoid this lookup
  topic.key_str = str(topic.key())
  return topic

# responds to /<forumurl>/topic?key=<key>
class TopicForm(IndexForm):

  def get(self):
    forum = forum_from_url(self.request.path_info)
    if not forum:
      return self.forum_list()

    topicid = self.request.get('key')

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
    template_out(self.response, "post.html", tvals)

  def invalid_captcha(self):
      template_out(self.response, "invalid_captcha.html", {})

  def post(self):
    forum = forum_from_url(self.request.path_info)
    if not forum:
      return self.forum_list()
    # TODO: read form values and act apropriately
    if self.request.get('Cancel'):
      # TODO: should redirect instead?
      return self.forum_index(forum)

    captcha = self.request.get('Captcha')
    captchaResponse = self.request.get('CaptchaResponse')
    try:
      captcha = int(captcha)
      captchaResponse = int(captchaResponse)
    except ValueError:
      return self.invalid_captcha()

    if captcha != captchaResponse:
      return self.invalid_captcha()

    subject = self.request.get('Subject').strip()
    msg = self.request.get('Message').strip()
    fullName = self.request.get('Name').strip()
    url = self.request.get('Url').strip()
    if url == "http://":
      url = ""
    # TODO: handle user names, urls
    # TODO: update comments count on Topic
    topic_key = self.request.get('Topic').strip()
    if not topic_key:
      topic = Topic(subject=subject)
      topic.put()
    else:
      assert 0 # not yet handled
      topic = Model.get_by_key_name(topic_key)
      topic.ncount += 1

    p = Post(body=msg, topic=topic)
    p.put()
    if not topic_key:
      topic.put()
    # TODO: redirect?
    self.redirect("/" + forum.url)
    #self.forum_index(forum)

class Dispatcher(IndexForm):

  def get(self):
    forum = forum_from_url(self.request.path_info)
    if not forum:
      return self.forum_list()

    self.forum_index(forum)

  def post(self):
    template_out(self.response, "404.html", {})  

def main():
  application = webapp.WSGIApplication( 
     [ ('/forumsmanage', ForumsManage),
       ('/[^/]*/post', PostForm),
       ('/[^/]*/topic', TopicForm),
       ('.*', Dispatcher)],
     debug=True)
  wsgiref.handlers.CGIHandler().run(application)

if __name__ == "__main__":
  main()
