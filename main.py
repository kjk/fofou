import os
import wsgiref.handlers
from google.appengine.ext import webapp
from google.appengine.ext.webapp import template

class Dispatcher(webapp.RequestHandler):
  def get(self):
    req = self.request
    path = req.path_info
    self.response.headers['Content-Type'] = 'text/html'
    template_values = {
      'title' : "Sumatra PDF forums",
      'path' : req.path_info
    }
    path = os.path.join(os.path.dirname(__file__), 'index.html')
    self.response.out.write(template.render(path, template_values))

def main():
  application = webapp.WSGIApplication( [('.*', Dispatcher)],
                                       debug=True)
  wsgiref.handlers.CGIHandler().run(application)

if __name__ == "__main__":
  main()
