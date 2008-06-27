import os, string, Cookie, sha, time, random, cgi, urllib
import wsgiref.handlers
from google.appengine.ext import db
from google.appengine.api import users
from google.appengine.ext import webapp
from google.appengine.ext.webapp import template
import logging

# TODO must have:
#  - show poster name and comment count in topic_list.html
#  - fix layout of search box
#  - import posts from a file (good enough to import fruitshow forums)
#  - determine topic.is_deleted (when deleting a post, delete a topic if all
#    posts are deleted. when undeleting a post - undelete a topic (if deleted))
#  - archives (by month?)
#  - write a web page for fofou
#  - hookup sumatra forums at fofou.org
#  - handle 'older topics' button
#  - determine is_archived
#  - /<forumurl>/email?postId=<id>
#  - /<forumurl>/moderate?del|undel=<postId>&ret=<returnUrl>
#  - after posting to existing topic, redirect to topic?id=<id> url
# TODO less urgent:
#  - /<forumurl>/rss - rss feed
#  - /<forumurl>/rssall - like /rss but shows all posts, not only when a
#  - /rsscombined - all posts for all forums, for forum admins mostly
#  - admin features like blocking users (ip address, cookie, user_id)
#  - per-forum templates
#  - use template inheritance to reduce duplication of html
#  - figure out why spacing between sections is so small (and fix it)
# Maybe:
#  - cookie validation
#  - alternative forms of integration with a wesite (iframe? return data
#    as json and do most of the rendering using javascrip?)

# Structure of urls:
#
# Top-level urls
#
# / - list of all forums
# /manageforums[?forum=<key> - edit/create/disable forums
#
# Per-forum urls
#
# /<forum_url>/
#    index, lists recent topics
#
# /<forum_url>/post[?id=<id>]
#    form for creating a new post. if "topic" is present, it's a post in
#    existing topic, otherwise a post starting a new topic
#
# /<forum_url>/topic?id=<id>&comments=<comments>
#    shows posts in a given topic, 'comments' is ignored (just a trick to re-use
#    browser's history to see if the topic has posts that user didn't see yet
#
# /<forum_url>/moderate?del=<postId>&ret=<url>
# /<forum_url>/moderate?undel=<postId>&ret=<url>
#
# /<forum_url>/rss
#    rss feed for first post in the topic (default)
#
# /<forum_url>/rssall
#    rss feed for all posts

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
  # admin can delete (and then undelete) topics
  is_deleted = db.BooleanProperty(default=False)
  # ncomments is redundant but is faster than always quering count of Posts
  ncomments = db.IntegerProperty(default=0)

# A topic is a collection of posts
class Post(db.Model):
  topic = db.Reference(Topic, required=True)
  created_on = db.DateTimeProperty(auto_now_add=True)
  message = db.TextProperty(required=True)
  # admin can delete/undelete posts
  is_deleted = db.BooleanProperty(default=False)
  # ip address from which this post has been made
  user_ip = db.StringProperty(required=True)
  user = db.Reference(FofouUser, required=True)
  # user_name, user_email and user_homepage might be different than
  # name/homepage/email fields in user object, since they can be changed in
  # FofouUser
  user_name = db.StringProperty()
  user_email = db.StringProperty()
  user_homepage = db.StringProperty()

# cookie code based on http://code.google.com/p/appengine-utitlies/source/browse/trunk/utilities/session.py
FOFOU_COOKIE = "fofou-uid"
COOKIE_EXPIRE_TIME = 60*60*24*120 # valid for 60*60*24*120 seconds => 120 days

def get_user_agent(): return os.environ['HTTP_USER_AGENT']
def get_remote_ip(): return os.environ['REMOTE_ADDR']

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

g_fofou_cookie = None
# returns either a FOFOU_COOKIE sent by the browser or a newly created cookie
def get_fofou_cookie():
  global g_fofou_cookie
  if g_fofou_cookie:
    return g_fofou_cookie
  cookies = get_inbound_cookie()
  for cookieName in cookies.keys():
    if FOFOU_COOKIE != cookieName:
      del cookies[cookieName]
  if (FOFOU_COOKIE not in cookies) or not valid_user_cookie(cookies[FOFOU_COOKIE].value):
    cookies[FOFOU_COOKIE] = new_user_id()
    cookies[FOFOU_COOKIE]['path'] = '/'
    cookies[FOFOU_COOKIE]['expires'] = COOKIE_EXPIRE_TIME
  g_fofou_cookie = cookies[FOFOU_COOKIE]
  return g_fofou_cookie

def get_fofou_cookie_val():
  c = get_fofou_cookie()
  return c.value

g_fofou_set_cookie = None
# remember cookie so that we can send it when we render a template
def send_fofou_cookie():
  global g_fofou_set_cookie
  if not g_fofou_set_cookie:
    g_fofou_set_cookie = get_fofou_cookie()

g_anonUser = None
def anonUser():
  global g_anonUser
  if None == g_anonUser:
    g_anonUser = users.User("dummy@dummy.address.com")
  return g_anonUser

def template_out(response, template_name, template_values):
  response.headers['Content-Type'] = 'text/html'
  if g_fofou_set_cookie:
    # a hack extract the cookie part from the whole "Set-Cookie: val" header
    c = str(g_fofou_set_cookie)
    c = c.split(": ", 1)[1]
    response.headers["Set-Cookie"] = c
  path = os.path.join(os.path.dirname(__file__), template_name)
  response.out.write(template.render(path, template_values))

def valid_forum_url(url):
  if not url:
    return False
  return url == urllib.quote_plus(url)

def valid_subject(txt):
  if not txt:
    return False
  return True

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

def get_log_in_out(url):
  user = users.get_current_user()
  if user:
    if users.is_current_user_admin():
      return "Welcome admin, %s! <a href=\"%s\">Log out</a>" % (user.nickname(), users.create_logout_url(url))
    else:
      return "Welcome, %s! <a href=\"%s\">Log out</a>" % (user.nickname(), users.create_logout_url(url))
  else:
    return "<a href=\"%s\">Log in or register</a>" % users.create_login_url(url)    

# from http://www.python.org/dev/peps/pep-0333/#url-reconstruction
def request_url():
  from urllib import quote
  url = os.environ['wsgi.url_scheme']+'://'

  if os.environ.get('HTTP_HOST'):
    url += os.environ['HTTP_HOST']
  else:
    url += os.environ['SERVER_NAME']

    if os.environ['wsgi.url_scheme'] == 'https':
      if os.environ['SERVER_PORT'] != '443':
         url += ':' + os.environ['SERVER_PORT']
    else:
      if os.environ['SERVER_PORT'] != '80':
         url += ':' + os.environ['SERVER_PORT']

    url += quote(os.environ.get('SCRIPT_NAME',''))
    url += quote(os.environ.get('PATH_INFO',''))
    return url

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
        'prevurl' : url,
        'prevtitle' : title,
        'prevtagline' : tagline,
        'prevsidebar' : sidebar,
        'forum_key' : forum_key,
        'errmsg' : "Url contains illegal characters"
      }
      return self.render_rest(tvals)

    title_or_url = title or url
    if forum:
      # update existing forum
      forum.url = url
      forum.title = title
      forum.tagline = tagline
      forum.sidebar = sidebar
      forum.put()
      msg = "Forum '%s' has been updated." % title_or_url
    else:
      # create a new forum
      forum = Forum(url=url, title=title, tagline=tagline, sidebar=sidebar)
      forum.put()
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
        tvals['forum_key'] = str(f.key())
      forums.append(f)
    tvals['msg'] = self.request.get('msg')
    tvals['user'] = user
    tvals['forums'] = forums
    template_out(self.response,  "manage_forums.html", tvals)

# responds to /, shows list of available forums or redirects to
# forum management page if user is admin
class ForumList(webapp.RequestHandler):
  def get(self):
    if users.is_current_user_admin():
      return self.redirect("/manageforums")
    MAX_FORUMS = 256 # if you need more, tough
    forums = db.GqlQuery("SELECT * FROM Forum").fetch(MAX_FORUMS)
    tvals = {
      'forums' : forums,
      'isadmin' : users.is_current_user_admin(),
      'log_in_out' : get_log_in_out("/")
    }
    template_out(self.response,  "forum_list.html", tvals)
  
# responds to /<forumurl>/, shows a list of recent topics
class TopicListForm(webapp.RequestHandler):
  def get(self):
    forum = forum_from_url(self.request.path_info)
    if not forum or forum.is_disabled:
      return self.redirect("/")
    siteroot = forum_root(forum)
    MAX_TOPICS = 25
    if users.is_current_user_admin():
      topics = Topic.gql("WHERE forum = :1 ORDER BY created_on DESC", forum).fetch(MAX_TOPICS)
    else:
      topics = Topic.gql("WHERE forum = :1 AND is_deleted = False ORDER BY created_on DESC", forum).fetch(MAX_TOPICS)

    tvals = {
      'siteroot' : siteroot,
      'siteurl' : self.request.url,
      'forum' : forum,
      'topics' : topics,
      'log_in_out' : get_log_in_out(siteroot)
    }
    template_out(self.response,  "topic_list.html", tvals)

# responds to /<forumurl>/topic?id=<id>
class TopicForm(webapp.RequestHandler):

  def get(self):
    forum = forum_from_url(self.request.path_info)
    if not forum:
      return self.redirect("/")
    siteroot = forum_root(forum)

    topic_id = self.request.get('id')
    if not topic_id:
      return self.redirect(siteroot)

    topic = db.get(db.Key.from_path('Topic', int(topic_id)))
    if not topic:
      return self.redirect(siteroot)

    if topic.is_deleted:
      # TODO: but not if is admin? (in which case we want to be able to 
      # undelete the topic). But then again, maybe that should be handled
      # in topic list view or a separate page that just lists deleted topics
      return self.redirect(siteroot)

    # TODO: decide if is archived
    is_archived = False

    # 200 is more than generous
    MAX_POSTS = 200
    if users.is_current_user_admin():
      posts = Post.gql("WHERE topic = :1 ORDER BY created_on", topic).fetch(MAX_POSTS)
    else:
      posts = Post.gql("WHERE topic = :1 AND is_deleted = False ORDER BY created_on", topic).fetch(MAX_POSTS)

    tvals = {
      'siteroot' : siteroot,
      'forum' : forum,
      'topic' : topic,
      'is_admin' : users.is_current_user_admin(),
      'is_archived' : is_archived,
      'posts' : posts,
      'log_in_out' : get_log_in_out(siteroot)
    }
    template_out(self.response, "topic.html", tvals)

# responds to /<forumurl>/rss, returns an RSS feed of recent topics
class RssFeed(webapp.RequestHandler):
  def get(self):
    forum = forum_from_url(self.request.path_info)
    if not forum:
      # TODO: return 404?
      return self.redirect("/")

def get_fofou_user():
  # get user either by google user id or cookie
  user_id = users.get_current_user()
  user = None
  if user_id:
    user = FofouUser.gql("WHERE user = :1", user_id).get()
    if user:
      logging.info("Found existing user for by user_id '%s'" % str(user_id))
  else:
    cookie = get_fofou_cookie_val()
    if cookie:
      user = FofouUser.gql("WHERE cookie = :1", cookie).get()
      if user:
        logging.info("Found existing user for cookie '%s'" % cookie)
      else:
        logging.info("Didn't find user for cookie '%s'" % cookie)
  return user

# responds to /<forumurl>/post[?id=<topic_id>]
class PostForm(webapp.RequestHandler):

  def get(self):
    forum = forum_from_url(self.request.path_info)
    if not forum or forum.is_disabled:
      return self.redirect("/")
    siteroot = forum_root(forum)
    send_fofou_cookie()

    rememberChecked = ""
    prevUrl = "http://"
    prevEmail = ""
    prevName = ""
    user = get_fofou_user()
    if user and user.remember_me:
      rememberChecked = "checked"
      prevUrl = user.homepage
      if not prevUrl:
        prevUrl = "http://"
      prevName = user.name
      prevEmail = user.email
    (num1, num2) = (random.randint(1,9), random.randint(1,9))
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
      'log_in_out' : get_log_in_out(siteroot + "post")
    }
    topic_id = self.request.get('id')
    if topic_id:
      topic = db.get(db.Key.from_path('Topic', int(topic_id)))
      if not topic:
        return self.redirect(siteroot)
      tvals['prevTopicId'] = topic_id
      tvals['prevSubject'] = topic.subject
    template_out(self.response, "post.html", tvals)

  def post(self):
    forum = forum_from_url(self.request.path_info)
    if not forum:
      return self.redirect("/")
    siteroot = forum_root(forum)

    if self.request.get('Cancel'):
      self.redirect(siteroot)

    send_fofou_cookie()

    topic_id = self.request.get('TopicId')
    num1 = self.request.get('num1')
    num2 = self.request.get('num2')
    captcha = self.request.get('Captcha').strip()
    subject = self.request.get('Subject')
    if subject: # TODO: do I need to check or can I just subject.strip()
      subject = subject.strip() 
    message = self.request.get('Message').strip()
    remember_me = self.request.get('Remember').strip()
    logging.info("Remember me is: '%s'" % remember_me)
    if remember_me == "":
      remember_me = False
    else:
      remember_me = True

    rememberChecked = ""
    if remember_me:
      rememberChecked = "checked"
    email = self.request.get('Email').strip()
    name = self.request.get('Name').strip()
    homepage = self.request.get('Url').strip()

    validCaptcha = True
    try:
      captcha = int(captcha)
      num1 = int(num1)
      num2 = int(num2)
    except ValueError:
      validCaptcha = False

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

    # ensure user properly answered math question
    if not validCaptcha or (captcha != (num1 + num2)):
      tvals['captcha_class'] = "error"
      return template_out(self.response, "post.html", tvals)

    # 'http://' is the default value we put, so if unchanged, consider it
    # as not given at all
    if homepage == "http://":
      homepage = ""

    # message cannot be empty
    if not message:
      tvals['message_class'] = "error"
      return template_out(self.response, "post.html", tvals)
    
    # name cannot be empty
    if not name:
      tvals['name_class'] = "error"
      return template_out(self.response, "post.html", tvals)
    
    if not valid_email(email):
      tvals['email_class'] = "error"
      return template_out(self.response, "post.html", tvals)

    # get user either by google user id or cookie. Create user objects if don't
    # already exist
    existing_user = False
    user_id = users.get_current_user()
    if user_id:
      user = FofouUser.gql("WHERE user = :1", user_id).get()
      if not user:
        logging.info("Creating new user for '%s'" % str(user_id))
        user = FofouUser(user=user_id, remember_me = remember_me, email=email, name=name, homepage=homepage)
        user.put()
      else:
        existing_user = True
        logging.info("Found existing user for '%s'" % str(user_id))
    else:
      cookie = get_fofou_cookie_val()
      user = FofouUser.gql("WHERE cookie = :1", cookie).get()
      if not user:
        logging.info("Creating new user for cookie '%s'" % cookie)
        user = FofouUser(cookie=cookie, remember_me = remember_me, email=email, name=name, homepage=homepage)
        user.put()
      else:
        existing_user = True
        logging.info("Found existing user for cookie '%s'" % cookie)

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
        logging.info("User needed an update")
        user.put()

    if not topic_id:
      # first post in a topic, so create the topic
      if not valid_subject(subject):
        tvals['subject_class'] = "error"
        return template_out(self.response, "post.html", tvals)
      topic = Topic(forum=forum, subject=subject, created_by=name)
      topic.put()
    else:
      topic = db.get(db.Key.from_path('Topic', int(topic_id)))
      if forum.key() != topic.forum.key():
        logging.info("forum.url      : %s" % forum.url)
        logging.info("topic.forum.url: %s" % topic.forum.url)
        logging.info("forum.key      : %s" % str(forum.key()))
        logging.info("topic.forum.key: %s" % str(topic.forum.key()))
      assert forum.key() == topic.forum.key()
      topic.ncomments += 1
      topic.put()

    user_ip = get_remote_ip()
    p = Post(user=user, user_ip=user_ip, topic=topic, message=message, user_name = name, user_email = email, user_homepage = homepage)
    p.put()
    if topic_id:
      self.redirect(siteroot + "topic?id=" + topic_id)
    else:
      self.redirect(siteroot)

def main():
  application = webapp.WSGIApplication(
     [  ('/', ForumList),
        ('/manageforums', ManageForums),
        ('/[^/]+/post', PostForm),
        ('/[^/]+/topic', TopicForm),
        ('/[^/]+/rss', RssFeed),
        ('/[^/]+/?', TopicListForm)],
     debug=True)
  wsgiref.handlers.CGIHandler().run(application)

if __name__ == "__main__":
  main()
