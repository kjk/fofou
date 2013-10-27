# the backup is in kjkbackup bucket, directory blog:
# - directory blobs has articles blobs
# - directory blobs_crashes has crashes blobs
# - $date_$hash.zip has the rest (the latest has the most recent info)
import os, json, zipfile


g_aws_access = None
g_aws_secret = None
g_bucket = "kjkbackup"
g_conn = None


def memoize(func):
	memory = {}
	def __decorated(*args):
		if args not in memory:
			memory[args] = func(*args)
		return memory[args]
	return __decorated


def s3_get_conn():
  global g_conn
  from boto.s3.connection import S3Connection
  if g_conn is None:
    g_conn = S3Connection(g_aws_access, g_aws_secret, True)
  return g_conn


def s3_get_bucket():
  return s3_get_conn().get_bucket(g_bucket)


def s3_list(s3_dir):
  from boto.s3.bucketlistresultset import bucket_lister
  b = s3_get_bucket()
  return bucket_lister(b, s3_dir)


def delete_file(path):
  if os.path.exists(path):
    os.remove(path)


def create_dir(d):
  if not os.path.exists(d): os.makedirs(d)
  return d

@memoize
def script_dir(): return os.path.realpath(os.path.dirname(__file__))


# where we will download the files
# to $script_dir/../../../fofoudata directory (if exists) - for local testing
# to the same directory where the script is - on the server
@memoize
def local_download_dir():
	d = os.path.join(script_dir(), "..", "..", "..", "fofoudata")
	if os.path.exists(d):
		return d
	return script_dir()


def get_config_json_path():
	d1 = script_dir()
	f_path = os.path.join(d1, "config.json")
	if os.path.exists(f_path):
		return f_path
	d2 = os.path.join(script_dir(), "..")
	f_path = os.path.join(d2, "config.json")
	if os.path.exists(f_path):
		return f_path
	assert False, "config.json not found in %s or %s" % (d1, d2)


def find_latest_zip(zip_files):
	sorted_by_name = sorted(zip_files, key=lambda el: el.name)
	#print(sorted_by_name)
	sorted_by_mod_time = sorted(zip_files, key=lambda el: el.last_modified)
	#print(sorted_by_mod_time)
	v1 = sorted_by_name[-1]
	v2 = sorted_by_mod_time[-1]
	assert v1 == v2, "inconsistency in zip files, %s != %s" % (str(v1), str(v2))
	return v1


def restore_from_zip(s3_key):
	print("Restoring backup files from s3 zip: %s" % s3_key.name)
	tmp_path = os.path.join(local_download_dir(), "tmp.zip")
	delete_file(tmp_path)
	s3_key.get_contents_to_filename(tmp_path)
	zf = zipfile.ZipFile(tmp_path, "r")
	dst_dir = os.path.join(local_download_dir(), "forum")
	create_dir(dst_dir)
	for name in zf.namelist():
		dst_path = os.path.join(dst_dir, name)
		delete_file(dst_path) # just in case
		zf.extract(name, dst_dir)
		print("  extracted %s to %s " % (name, dst_path))
	delete_file(tmp_path)


# limit is for testing, 0 means no limit
def restore_blobs(s3_keys, s3_prefix, relative_dst_dir, limit=0):
	print("Restoring %d blobs with s3_prefix '%s' to dir '%s'" % (len(s3_keys), s3_prefix, relative_dst_dir))
	restored = 0
	restored_with_existing = 0
	for key in s3_keys:
		restored_with_existing += 1
		assert key.name.startswith(s3_prefix)
		name = key.name[len(s3_prefix):]
		dst_path = os.path.join(local_download_dir(), relative_dst_dir, name)
		if os.path.exists(dst_path):
			# TODO: could check sha1 as well
			print("  %s already restored" % dst_path)
			continue
		# not sure if boto creates the dir, so ensure destination dir exists
		print("  downloading %s => %s" % (key.name, dst_path))
		dst_dir = os.path.dirname(dst_path)
		#print("  dst_path = '%s' dst_dir = '%s'" % (dst_path, dst_dir))
		create_dir(dst_dir)
		key.get_contents_to_filename(dst_path)
		restored += 1
		if limit != 0 and restored >= limit:
			return
		if restored % 100 == 0:
			left = len(s3_keys) - restored_with_existing
			print(" left: %d, restored %d" % (left, restored))


def main():
	global g_aws_access, g_aws_secret
	print("Will download to %s" % local_download_dir())
	f_path = get_config_json_path()
	#print(f_path)
	d = open(f_path).read()
	d = json.loads(d)
	g_aws_access = d["AwsAccess"]
	g_aws_secret = d["AwsSecret"]
	print("Listing files in s3...")
	files = s3_list("fofou")
	zip_files = []
	blobs_files = []
	n = 0
	for f in files:
		n += 1
		if n % 1000 == 0:
			print("%d files in s3" % n)
		name = f.name
		if name.endswith(".zip"):
			zip_files.append(f)
		elif name.startswith("fofou/blobs/"):
			blobs_files.append(f)
			#if len(blobs_files) > 100: break # for testing
		else:
			assert False, "%s (%s) is unrecognized files in s3" % (str(f), name)
	print("%d zip files, %d blobs" % (len(zip_files), len(blobs_files)))
	latest_zip = find_latest_zip(zip_files)
	restore_from_zip(latest_zip)
	restore_blobs(blobs_files, "fofou/blobs/", "blobs")


if __name__ == "__main__":
	main()
