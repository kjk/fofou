import pickle, bz2, os.path

PICKLED_DATA_FILE_NAME = "fruitshow_posts.dat.bz2"

(POST_ID, POST_MSG, POST_NAME, POST_EMAIL, POST_URL, POST_POSTED_ON, POST_POSTER_IP, POST_POSTER_KEY, POST_UNIQUE_KEY, POST_DELETED, POST_LAST_MOD) = range(11)
(TOPIC_ID, TOPIC_FIRST_POST, TOPIC_SUBJECT) = range(3)
(TP_TOPIC_ID, TP_POST_ID) = range(2)

def main():
  if not os.path.exists(PICKLED_DATA_FILE_NAME):
    print("File %s doesn't exists" % PICKLED_DATA_FILE_NAME)
    return
  fo = bz2.BZ2File(PICKLED_DATA_FILE_NAME, "r")
  data = pickle.load(fo)
  fo.close()
  all_topics = data["topics"]
  all_posts = data["posts"]
  topic_posts = data["topic_posts"]
  print("%d topics" % len(all_topics))
  print("%d posts" % len(all_posts))
  print("%d topic_posts" % len(topic_posts))

  for topic in all_topics:
    topic_id = topic[TOPIC_ID]
    topic_first_post = topic[TOPIC_FIRST_POST]
    subject = topic[TOPIC_SUBJECT]
    print("Subject: '%s'" % subject)
    post_ids = [p[TP_POST_ID] for p in topic_posts if topic_id == p[TP_TOPIC_ID]]
    print post_ids
    posts = [p for p in all_posts if p[POST_ID] in post_ids]
    print posts
    break

if __name__ == "__main__":
  main()

