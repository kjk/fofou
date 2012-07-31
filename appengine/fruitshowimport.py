# This file is obsolete and shows how you can add a way to import
# existing posts from some other forum software
# see fruitshow_dump_data.py and fruitshow_dump_upload.py to see
# how posts were extracted and submitted here

# responds to /<forumurl>/importfruitshow
class ImportFruitshow(webapp.RequestHandler):

  def post(self):
    (forum, siteroot, tmpldir) = forum_siteroot_tmpldir_from_url(self.request.path_info)
    if not forum or forum.is_disabled:
      return self.error(HTTP_NOT_ACCEPTABLE)
    # not active at all if not protected by secret
    if not forum.import_secret:
      logging.info("tried to import topic into '%s' forum, but forum has no import_secret" % forum.url)
      return self.error(HTTP_NOT_ACCEPTABLE)
    (topic_pickled, import_secret) = req_get_vals(self.request, ["topicdata", 'importsecret'], strip=False)
    if not topic_pickled:
      logging.info("tried to import topic into '%s' forum, but no 'topicdata' field" % forum.url)
      return self.error(HTTP_NOT_ACCEPTABLE)
    if import_secret != forum.import_secret:
        logging.info("tried to import topic into '%s' forum, but import_secret doesn't match" % forum.url)
        return self.error(HTTP_NOT_ACCEPTABLE)
      
    fo = StringIO.StringIO(topic_pickled)
    topic_data = pickle.load(fo)
    fo.close()

    (topic, posts) = topic_data
    topic_no = topic[TOPIC_ID]
    if 0 == len(posts):
      logging.info("There are no posts in this topic.")
      return self.error(HTTP_NOT_ACCEPTABLE)

    subject = to_unicode(topic[TOPIC_SUBJECT])
    first_post = posts[0]
    last_post = posts[-1]
    created_on = first_post[POST_POSTED_ON]
    #logging.info("subject: %s, created_on: %s" % (subject, str(created_on)))
    topic = Topic.gql("WHERE forum = :1 AND subject = :2 AND created_on = :3", forum, subject, created_on).get()
    if topic:
      logging.info("topic already exists, subject: %s, created_on: %s" % (subject, str(created_on)))
      return self.error(HTTP_NOT_ACCEPTABLE)
    created_by = to_unicode(first_post[POST_NAME])
    topic = Topic(forum=forum, subject=subject, created_on=created_on, created_by=created_by, updated_on = created_on)
    topic.ncomments = len(posts)-1
    topic.updated_on = last_post[POST_POSTED_ON]
    topic.is_deleted = bool(int(first_post[POST_DELETED]))
    topic.put()
    #logging.info("created topic, subject: %s, created_on: %s" % (subject, str(created_on)))
    for post in posts:
      body = to_unicode(post[POST_MSG])
      name = to_unicode(post[POST_NAME])
      email = post[POST_EMAIL]
      homepage = post[POST_URL]
      (name, email, homepage) = (name.strip(), email.strip(), homepage.strip())
      if len(homepage) <= len("http://"):
        homepage = ""
      created_on = post[POST_POSTED_ON]
      # this is already an integer, not string
      user_ip = post[POST_POSTER_IP]
      is_deleted = bool(int(post[POST_DELETED]))
      user = FofouUser.gql("WHERE name = :1 AND email = :2 AND homepage = :3", name, email, homepage).get()
      if not user:
        #logging.info("Didn't find user for name='%s', email='%s', homepage='%s'. Creating one." % (name, email, homepage))
        cookie = new_user_id()
        user = FofouUser(cookie=cookie, name=name, email=email, homepage=homepage)
        user.put()

      # sha.new() doesn't accept Unicode strings, so convert to utf8 first
      body_utf8 = body.encode('UTF-8')
      s = sha.new(body_utf8)
      sha1_digest = s.hexdigest()
      new_post = Post(topic=topic, forum=forum, created_on=created_on, message=body, sha1_digest=sha1_digest, is_deleted=is_deleted, user_ip=user_ip, user=user)
      new_post.user_name = name
      new_post.user_email = email
      new_post.user_homepage = homepage
      new_post.put()
      logging.info("Imported post %s" % str(post[POST_ID]))
    logging.info("Imported topic %s" % str(topic_no))

