import pickle, bz2, os.path, string, urlparse, httplib, traceback, StringIO
from offsets import *

# Uploads posts dumped with fruitshow_dump_data.py to fofou

# you need to provide full url to a given forum's posting interface e.g.
# http://foo.com/myforum/importtopic
#FOFOU_SERVER = None
FOFOU_SERVER = "http://localhost:9999/sumatrapdf/importtopic"

PICKLED_DATA_FILE_NAME = "fruitshow_posts.dat.bz2"

def encode_multipart_formdata(fields, files):
    """
    fields is a sequence of (name, value) elements for regular form fields.
    files is a sequence of (name, filename, value) elements for data to be uploaded as files
    Return (content_type, body) ready for httplib.HTTP instance
    """
    BOUNDARY = '----------ThIs_Is_tHe_bouNdaRY_$'
    CRLF = '\r\n'
    L = []
    assert fields or files
    if fields:
        for (key, value) in fields:
            L.append('--' + BOUNDARY)
            L.append('Content-Disposition: form-data; name="%s"' % key)
            L.append('')
            L.append(value)
    if files:
        for (key, filename, value) in files:
            L.append('--' + BOUNDARY)
            L.append('Content-Disposition: form-data; name="%s"; filename="%s"' % (key, filename))
            L.append('Content-Type: %s' % get_content_type(filename))
            L.append('')
            L.append(value)
    L.append('--' + BOUNDARY + '--')
    L.append('')
    body = CRLF.join(L)
    content_type = 'multipart/form-data; boundary=%s' % BOUNDARY
    return content_type, body

# from http://aspn.activestate.com/ASPN/Cookbook/Python/Recipe/146306
def post_multipart(host, selector, fields, files, username=None, pwd=None):
    """
    Post fields and files to an http host as multipart/form-data.
    fields is a sequence of (name, value) elements for regular form fields.
    files is a sequence of (name, filename, value) elements for data to be uploaded as files
    Return the server's response page.
    """
    print "post_multipart selector=%s" % selector
    data = None
    try:
        content_type, body = encode_multipart_formdata(fields, files)
        conn = httplib.HTTPConnection(host)
        conn.putrequest('POST', selector)
        conn.putheader('Content-Type', content_type)
        conn.putheader('Content-Length', str(len(body)))
        if username:
            assert(pwd)
            conn.putheader('Authorization', "Basic %s" % gen_base_auth_header_val(username, pwd))
        conn.endheaders()
        #print "post_multipart() body: '%s'" % body
        conn.send(body)
        resp = conn.getresponse()
        #err = resp.status
        data = resp.read()
    except:
        print "post_multipart failed"
        print '-'*60
        traceback.print_exc(file=sys.stdout)
        print '-'*60
    #print data
    return data

def do_http_post_fields(url, field_name, field_data, username=None, pwd=None):
    print "do_http_post_fields url=%s" % url
    url_parts = urlparse.urlparse(url)
    host = url_parts.netloc
    selector = url_parts.path
    if url_parts.query:
        selector = selector + "?" + url_parts.query
    fields = [(field_name, field_data)]
    return post_multipart(host, selector, fields, None, username, pwd)

def upload_post(url, topic_txt):
  print(topic_txt)
  do_http_post_fields(url, "topicdata", topic_txt)

def main():
  if not FOFOU_SERVER:
    print("You need to set FOFOU_SERVER")
    return
  if "/importtopic" not in FOFOU_SERVER:
    print("FOFOU_SERVER url ('%s') doesn't look valid (doesn't end with '/importtopic')" % FOFOU_SERVER)
    return
  if not os.path.exists(PICKLED_DATA_FILE_NAME):
    print("File %s doesn't exists" % PICKLED_DATA_FILE_NAME)
    return
  print("Reading '%s'" % PICKLED_DATA_FILE_NAME)
  fo = bz2.BZ2File(PICKLED_DATA_FILE_NAME, "r")
  data = pickle.load(fo)
  fo.close()
  print("Finished reading")
  all_topics = data["topics"]
  all_posts = data["posts"]
  topic_posts = data["topic_posts"]
  print("%d topics" % len(all_topics))
  print("%d posts" % len(all_posts))
  print("%d topic_posts" % len(topic_posts))

  for topic in all_topics:
    #txt = []
    topic_id = topic[TOPIC_ID]
    #topic_first_post = topic[TOPIC_FIRST_POST]
    #subject = topic[TOPIC_SUBJECT]
    #txt.append("Subject: '%s'" % subject)
    post_ids = [p[TP_POST_ID] for p in topic_posts if topic_id == p[TP_TOPIC_ID]]
    #print post_ids
    posts = [p for p in all_posts if p[POST_ID] in post_ids]
    topic_data = TopicData(topic, posts)
    fo = StringIO.StringIO()
    pickle.dump(topic_data, fo)
    topic_pickled = fo.getvalue()
    fo.close()
    upload_post(FOFOU_SERVER, topic_pickled)
    break

"""
    for post in posts:
      deleted = post[POST_DELETED]
      txt.append("PostCreatedOn: " + str(p[POST_POSTED_ON])
      txt.append("PostDeleted: " + str(int(deleted)))
      txt.append("PostBody:")
      body = post[POST_MSG]
      txt.append(body)
"""
    

if __name__ == "__main__":
  main()

