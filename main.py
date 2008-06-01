import os, string
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
# /post
#    form for creating a new post
# /topic/<id>?posts=<n>
#    shows posts in a given topic, 'posts' is ignored (just a trick to re-use
#    browser's history to see if the topic has posts that user didn't see yet
# /rss
#    rss feed for posts

class Forum(db.Model):
  # Urls for forums are in the form /<urlpart>/<rest>
  urlpart = db.StringProperty(required=True)
  # What we show as html <title>
  title = db.StringProperty()

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
      forums = None
      if len(self.forums) > 0:
        forums = self.forums
      tvals['nickname'] = self.user.nickname()
      tvals['forums'] = forums
    else:
      tname = "no_forums_not_admin.html"
      tvals['logouturl'] = users.create_logout_url(self.request.uri)
    path = os.path.join(os.path.dirname(__file__), tname)
    self.response.out.write(template.render(path, tvals))

  def get(self):
    # TODO: do I need to cache it or is it cached by the system?
    self.user = users.get_current_user()
    req = self.request
    path = req.path_info
    (forumurl, pathrest) = string.split(path, "/", 2)

    forumsq = db.GqlQuery("SELECT * FROM Forum")
    self.forums = []
    for f in forumsq:
      self.forums.append(f)
    forumcount = len(self.forums)
    if 0 == forumcount:
      return self.no_forums()

    self.response.headers['Content-Type'] = 'text/html'
    template_values = {
      'title' : "Sumatra PDF forums",
      'forum' : forumurl,
      'rest' : pathrest,
      'forumcount' : forumcount
    }
    path = os.path.join(os.path.dirname(__file__), 'index.html')
    self.response.out.write(template.render(path, template_values))

def main():
  application = webapp.WSGIApplication( [('.*', Dispatcher)],
                                       debug=True)
  wsgiref.handlers.CGIHandler().run(application)

if __name__ == "__main__":
  main()
