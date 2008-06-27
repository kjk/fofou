#!/usr/bin/env python
import MySQLdb, csv

# set the database connection properties
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

(TOPIC_ID, TOPIC_FIRST_POST, TOPIC_SUBJECT) = range(3)

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

(POST_ID, POST_MSG, POST_NAME, POST_EMAIL, POST_URL, POST_POSTED_ON, POST_POSTER_IP, POST_POSTER_KEY, POST_UNIQUE_KEY, POST_DELETED, POST_LAST_MOD) = range(11)

"""
    `TopicId` int(11) unsigned NOT NULL default '0',
    `PostId` int(11) unsigned NOT NULL default '0'"""

(TP_TOPIC_ID, TP_POST_ID) = range(2)

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
  topics = get_topics()
  posts = get_posts()
  topic_posts = get_topic_posts()
  print("%d topics" % len(topics))
  print("%d posts" % len(posts))
  print("%d topic_posts" % len(topic_posts))
  conn_close()

if __name__ == "__main__":
  main()

