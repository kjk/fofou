# This code is in Public Domain. Take all the code you want, we'll just write more.

# TODO: rename this file to common.py or sth.

(POST_ID, POST_MSG, POST_NAME, POST_EMAIL, POST_URL, POST_POSTED_ON, POST_POSTER_IP, POST_POSTER_KEY, POST_UNIQUE_KEY, POST_DELETED, POST_LAST_MOD) = range(11)
(TOPIC_ID, TOPIC_FIRST_POST, TOPIC_SUBJECT) = range(3)
(TP_TOPIC_ID, TP_POST_ID) = range(2)

class TopicData:
  def __init__(self, topic, posts):
    self.topic = topic
    self.posts = posts

