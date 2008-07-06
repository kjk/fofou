# This code is in Public Domain. Take all the code you want, we'll just write more.
import pickle, bz2, os.path, string, urlparse, httplib, traceback, StringIO, datetime, sys
from offsets import *

# Uploads posts dumped with fruitshow_dump_data.py to fofou

# you need to provide full url to a given forum's posting interface e.g.
# http://foo.com/myforum/importfruitshow
FOFOU_SERVER = ""
# You need to provide import secret for this forum (can be set by forum
# admin in forum management web page)
IMPORT_SECRET = ""

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
    #print("post_multipart selector=%s" % selector)
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

def do_http_post_fields(url, fields, username=None, pwd=None):
    #print("do_http_post_fields url=%s" % url)
    url_parts = urlparse.urlparse(url)
    host = url_parts.netloc
    selector = url_parts.path
    if url_parts.query:
        selector = selector + "?" + url_parts.query
    return post_multipart(host, selector, fields, None, username, pwd)

def upload_post(url, topic_txt):
  fields = [('topicdata', topic_txt), ('importsecret', IMPORT_SECRET)]
  do_http_post_fields(url, fields)

def to_datetime(val):
  #print("type of '%s' is '%s'" % (str(val), type(val)))
  dt = datetime.datetime.utcfromtimestamp(val)
  #print("'%s' is '%s'" % (str(val), dt.isoformat()))
  return dt

def main():
  if not FOFOU_SERVER:
    print("You need to set FOFOU_SERVER")
    return
  if not IMPORT_SECRET:
    print("You need to set IMPORT_SECRET")
    return
  if "/importfruitshow" not in FOFOU_SERVER:
    print("FOFOU_SERVER url ('%s') doesn't look valid (doesn't end with '/importfruitshow')" % FOFOU_SERVER)
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

  # convert dates from numeric value to datetime instance, as required by
  # /topicimport interface and post body to unicode
  all_posts = data["posts"]
  new_all_posts = []
  for p in all_posts:
    new_p = [el for el in p]
    new_p[POST_POSTED_ON] = to_datetime(new_p[POST_POSTED_ON])
    #new_p[POST_MSG] = to_unicode(new_p[POST_MSG])
    new_all_posts.append(new_p)
  all_posts = new_all_posts
  topic_posts = data["topic_posts"]
  topics_count = len(all_topics)
  print("%d topics, %d posts" % (topics_count, len(all_posts)))
  sent = 0
  for topic in all_topics:
    topic_id = topic[TOPIC_ID]
    post_ids = [p[TP_POST_ID] for p in topic_posts if topic_id == p[TP_TOPIC_ID]]
    posts = [p for p in all_posts if p[POST_ID] in post_ids]
    topic_data = (topic, posts)
    fo = StringIO.StringIO()
    pickle.dump(topic_data, fo)
    topic_pickled = fo.getvalue()
    fo.close()
    #print("Subject: '%s'" % topic[TOPIC_SUBJECT])
    #print("uploading topic %s" % topic_id)
    sent = sent + 1
    deleted = posts[0][POST_DELETED]
    print("Uploading topic %d (out of %d), deleted = '%s'" % (sent, topics_count, str(deleted)))
    upload_post(FOFOU_SERVER, topic_pickled)
    #break

if __name__ == "__main__":
  main()

