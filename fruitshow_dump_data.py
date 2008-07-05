# This code is in Public Domain. Take all the code you want, we'll just write more.
#!/usr/bin/env python
import MySQLdb, bz2, pickle, os.path
from offsets import *

# given connection details to fruitshow mysql database, dumps the data into
# pickled and bzip2ed file that can be used by fruitshow_dump_upload.py
# to import the posts into fofou

PICKLED_DATA_FILE_NAME = "fruitshow_posts.dat.bz2"

# you need to provide connection info to fruitshow mysql database with
# permissions to query data
user = ""
host = ""
passwd = ""
db = ""

g_conn = None
def get_conn():
  global g_conn
  if not g_conn:
    g_conn = MySQLdb.connect(host=host, user=user, passwd=passwd, db=db)
  return g_conn

def conn_close():
  global g_conn
  if g_conn:
    g_conn.close()
    g_conn = None

"""   `TopicId` int(11) unsigned NOT NULL auto_increment,
      `FirstPostId` int(11) unsigned NOT NULL default '0',
      `Subject` varchar(64) NOT NULL default '',"""

"""  
  `PostId` int(11) unsigned NOT NULL auto_increment,
  `Message` longtext NOT NULL,
  `Name` varchar(64) NOT NULL default '',
  `Email` varchar(64) NOT NULL default '',
  `Url` varchar(128) NOT NULL default '',
  `PostedOn` int(11) unsigned NOT NULL default '0',
  `PosterIp` int(11) NOT NULL default '0',
  `PosterKey` varchar(32) NOT NULL default '',
  `UniqueKey` varchar(32) NOT NULL default '',
  `Deleted` tinyint(1) unsigned NOT NULL default '0',
  `LastModeratedBy` int(11) unsigned default NULL,"""

"""
    `TopicId` int(11) unsigned NOT NULL default '0',
    `PostId` int(11) unsigned NOT NULL default '0'"""

def get_from_query(query):
  res = []
  conn = get_conn()
  topicsc = conn.cursor()
  topicsc.execute(query)
  while True:
    row = topicsc.fetchone()
    if not row:
      break
    res.append(row)
  return res

def get_topics(): return get_from_query("SELECT * From Topic")
def get_posts(): return get_from_query("SELECT * From Post")
def get_topic_posts(): return get_from_query("SELECT * FROM TopicPost")

def main():
  if os.path.exists(PICKLED_DATA_FILE_NAME):
    print "File %s already exists" % PICKLED_DATA_FILE_NAME
    return
  topics = get_topics()
  posts = get_posts()
  topic_posts = get_topic_posts()
  conn_close()
  data = {}
  data["topics"] = topics
  data["posts"] = posts
  data["topic_posts"] = topic_posts
  print("%d topics" % len(topics))
  print("%d posts" % len(posts))
  print("%d topic_posts" % len(topic_posts))
  fo = bz2.BZ2File(PICKLED_DATA_FILE_NAME, "w")
  pickle.dump(data, fo)
  fo.close()
  print("Pickled fruitshow data to file '%s'" % PICKLED_DATA_FILE_NAME)

if __name__ == "__main__":

  main()

