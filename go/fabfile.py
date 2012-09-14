import sys, os, os.path, subprocess
import zipfile
from fabric.api import *
from fabric.contrib import *

# Deploys a new version of fofou to the server

env.hosts = ['test.fofou.org']
env.user = 'fofou'

def git_ensure_clean():
	out = subprocess.check_output(["git", "status", "--porcelain"])
	if len(out) != 0:
		print("won't deploy because repo has uncommitted changes:")
		print(out)
		sys.exit(1)

def git_pull():
	local("git pull")

def git_trunk_sha1():
	# TODO: use "git rev-parse origin" instead?
	return subprocess.check_output(["git", "log", "-1", "--pretty=format:%H"])

def delete_file(p):
	if os.path.exists(p):
		os.remove(p)

def ensure_remote_dir_exists(p):
	if not files.exists(p):
		abort("dir '%s' doesn't exist on remote server" % p)
	#with settings(warn_only=True):
	#	if run("test -d %s" % p).failed:
	#		abort("dir '%s' doesn't exist on remote server" % p)

def ensure_remote_file_exists(p):
	if not files.exists(p):
		abort("dir '%s' doesn't exist on remote server" % p)
	#with settings(warn_only=True):
	#	if run("test -f %s" % p).failed:
	#		abort("file '%s' doesn't exist on remote server" % p	)

def add_dir_files(zip_file, dir):
	for (path, dirs, files) in os.walk(dir):
		for f in files:
			p = os.path.join(path, f)
			zip_file.write(p)

def zip_files(zip_path):
	zf = zipfile.ZipFile(zip_path, mode="w", compression=zipfile.ZIP_DEFLATED)
	blacklist = []
	files = [f for f in os.listdir(".") if f.endswith(".go") and not f in blacklist]
	for f in files: zf.write(f)
	zf.write("secrets.json")
	add_dir_files(zf, "scripts")
	add_dir_files(zf, "ext")
	add_dir_files(zf, "tmpl")
	add_dir_files(zf, "static")
	zf.close()

def deploy():
	if not os.path.exists("secrets.json"): abort("secrets.json doesn't exist locally")
	#git_pull()
	git_ensure_clean()
	local("./scripts/build.sh")
	ensure_remote_dir_exists('www/app')
	ensure_remote_file_exists('www/data/sumatrapdf')
	sha1 = git_trunk_sha1()
	code_path_remote = 'www/app/' + sha1
	if files.exists(code_path_remote):
		abort('code for revision %s already exists on the server' % sha1)
	zip_path = sha1 + ".zip"
	zip_files(zip_path)
	zip_path_remote = 'www/app/' + zip_path
	put(zip_path, zip_path_remote)
	delete_file(zip_path)
	with cd('www/app'):
		run('unzip -q -x %s -d %s' % (zip_path, sha1))
		run('rm -f %s' % zip_path)
	# make sure it can build
	with cd(code_path_remote):
		run("./scripts/build.sh")

	curr_dir = 'www/app/current'
	if files.exists(curr_dir):
		# shut-down currently running instance
		with cd(curr_dir):
			run("/sbin/start-stop-daemon --stop --oknodo --exec fofou_app")
		# rename old current as prev for easy rollback of bad deploy
		with cd('www/app'):
			run('rm -f prev')
			run('mv current prev')

	# make this version current
	with cd('www/app'):
		run("ln -s %s current" % sha1)

	# start it
	with cd(curr_dir):
		run("/sbin/start-stop-daemon --start --background --chdir /home/fofou/www/app/current --exec fofou_app -- --log fofou_app.log")
		run("ps aux | grep _app")
