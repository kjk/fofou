import os, string, Cookie, sha, time, random, cgi, urllib
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
# /manageforums[?forum=<key> - edit/create/disable forums
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

# A forum is collection of topics, shown in the 'newest topic on top' order
class Topic(db.Model):
  forum = db.Reference(Forum, required=True)
  subject = db.StringProperty(required=True)
  created_on = db.DateTimeProperty(auto_now_add=True)
  # just in case, not used
  updated_on = db.DateTimeProperty(auto_now=True)
  # admin can delete (and then undelete) topics
  is_deleted = db.BooleanProperty(default=False)
  # ncomments is redundant but is faster than always quering count of Posts
  ncomments = db.IntegerProperty(default=0)

# A topic is a collection of posts
class Post(db.Model):
  topic = db.Reference(Topic, required=True)
  created_on = db.DateTimeProperty(auto_now_add=True)
  body = db.TextProperty(required=True)
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

def valid_forum_url(url):
  if not url:
    return False
  return url == urllib.quote_plus(url)

# responds to GET /manageforums[?forum=<key>&disable=yes&enable=yes]
# and POST /manageforums with values from the form
class ManageForums(webapp.RequestHandler):
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

    url = self.request.get('url')
    if url: url = url.strip()
    title = self.request.get('title')
    tagline = self.request.get('tagline')
    sidebar = self.request.get('sidebar')
    disable = self.request.get('disable')
    enable = self.request.get('enable')

    if not valid_forum_url(url):
      tvals = {
        # TODO: the value is passed in but apparently my form is not in the right form
        # need to figure this out
        'urlclass' : "error",
        'prevurl' : cgi.escape(url, True),
        'prevtitle' : cgi.escape(title, True),
        'prevtagline' : cgi.escape(tagline, True),
        'prevsidebar' : cgi.escape(sidebar, True),
        'forum_key' : forum_key,
        'errmsg' : "Url contains illegal characters"
      }
      return self.render_rest(tvals)

    if forum:
      # update existing forum
      forum.url = url
      forum.title = title
      forum.tagline = tagline
      forum.sidebar = sidebar
      forum.put()
      title_or_url = forum.title or forum.url
      msg = "Forum '%s' has been updated." % title_or_url
    else:
      # create a new forum
      forum = Forum(url=url, title=title, tagline=tagline, sidebar=sidebar)
      forum.put()
      title_or_url = title or url
      msg = "Forum '%s' has been created." % title_or_url
    url = "/manageforums?msg=%s" % urllib.quote(msg)
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

    tvals = {}
    if forum:
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
        return self.redirect("/manageforums?msg=%s" % urllib.quote(msg))
    self.render_rest(tvals, forum)

  def render_rest(self, tvals, forum=None):
    user = users.get_current_user()
    forumsq = db.GqlQuery("SELECT * FROM Forum")
    forums = []
    for f in forumsq:
      f.title_or_url = f.title or f.url
      f.edit_url = "/manageforums?forum_key=" + str(f.key())
      if f.is_disabled:
        f.enable_disable_txt = "enable"
        f.enable_disable_url = f.edit_url + "&enable=yes"
      else:
        f.enable_disable_txt = "disable"
        f.enable_disable_url = f.edit_url + "&disable=yes"      
      if forum and f.key() == forum.key():
        # editing existing forum
        f.no_edit_link = True
        tvals['prevurl'] = cgi.escape(f.url, True)
        tvals['prevtitle'] = cgi.escape(f.title, True)
        tvals['prevtagline'] = cgi.escape(f.tagline, True)
        tvals['prevsidebar'] = cgi.escape(f.sidebar, True)
        tvals['forum_key'] = str(f.key())
      forums.append(f)
    tvals['msg'] = self.request.get('msg')
    tvals['user'] = user
    tvals['forums'] = forums
    template_out(self.response,  "manage_forums.html", tvals)

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

  def forum_list(self):
    if users.is_current_user_admin():
      return self.redirect("/manageforums")
    forumsq = db.GqlQuery("SELECT * FROM Forum")
    forums = []
    for f in forumsq:
      f.title_or_url = f.title or f.url
      forums.append(f)
    tvals = {
      'forums' : forums,
    }
    user = users.get_current_user()
    if not user:
      tvals['loginurl'] = users.create_login_url(self.request.uri)
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
      'sidebar' : forum.sidebar,
      'tagline' : forum.tagline,
      'topics' : topics
    }
    template_out(self.response,  "index.html", tvals)

class ForumList(IndexForm):
  def get(self):
    return self.forum_list()

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
    if not forum or forum.is_disabled:
      return self.redirect("/")

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
    if not forum or forum.is_disabled:
      return self.redirect("/")

    self.forum_index(forum)

  def post(self):
    template_out(self.response, "404.html", {})  

def main():
  application = webapp.WSGIApplication(
     [ ('/', ForumList),
       ('/manageforums', ManageForums),
       ('/[^/]*/post', PostForm),
       ('/[^/]*/topic', TopicForm),
       ('.*', Dispatcher)],
     debug=True)
  wsgiref.handlers.CGIHandler().run(application)

if __name__ == "__main__":
  main()
