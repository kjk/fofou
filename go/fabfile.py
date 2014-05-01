import sys, os, os.path, subprocess
import zipfile
from fabric.api import *
from fabric.contrib import *


env.hosts = ['test.fofou.org']
env.user = 'fofou'

app_dir = 'www/app'


def git_ensure_clean():
	out = subprocess.check_output(["git", "status", "--porcelain"])
	if len(out) != 0:
		print("won't deploy because repo has uncommitted changes:")
		print(out)
		sys.exit(1)


def git_pull():
	local("git pull")


def git_trunk_sha1():
	return subprocess.check_output(["git", "log", "-1", "--pretty=format:%H"])


def delete_file(p):
	if os.path.exists(p):
		os.remove(p)


def ensure_remote_dir_exists(p):
	if not files.exists(p):
		abort("dir '%s' doesn't exist on remote server" % p)


def ensure_remote_file_exists(p):
	if not files.exists(p):
		abort("dir '%s' doesn't exist on remote server" % p)


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
	zf.write("config.json")
	add_dir_files(zf, "ext")
	add_dir_files(zf, "forums")
	add_dir_files(zf, "img")
	add_dir_files(zf, "scripts")
	add_dir_files(zf, "static")
	add_dir_files(zf, "tmpl")
	zf.close()


def delete_old_deploys(to_keep=5):
	with cd(app_dir):
		out = run('ls -1trF')
		lines = out.split("\n")
		i = 0
		dirs_to_del = []
		while i < len(lines):
			s = lines[i].strip()
			# extra precaution: skip dirs right after "prev@", "current@", they
			# are presumed to be their symlink targets
			if s in ["prev@", "current@"]:
				i += 1
				to_keep -= 1
			else:
				if len(s) == 41: # s == "0111cb7bdd014850e8c11ee4820dc0d7e12f4015/"
					dirs_to_del.append(s)
			i += 1
		if len(dirs_to_del) > to_keep:
			dirs_to_del = dirs_to_del[:-to_keep]
			print("deleting old deploys: %s" % str(dirs_to_del))
			for d in dirs_to_del:
				run("rm -rf %s" % d)


def check_config():
	if not os.path.exists("config.json"):
		abort("config.json doesn't exist locally")


def deploy():
	check_config()
	#git_pull()
	git_ensure_clean()
	local("./scripts/build.sh")
	local("./scripts/tests.sh")
	ensure_remote_dir_exists(app_dir)
	ensure_remote_file_exists('www/data')
	sha1 = git_trunk_sha1()
	code_path_remote = app_dir + '/' + sha1
	if files.exists(code_path_remote):
		abort('code for revision %s already exists on the server' % sha1)
	zip_path = sha1 + ".zip"
	zip_files(zip_path)
	zip_path_remote = app_dir + '/' + zip_path
	put(zip_path, zip_path_remote)
	delete_file(zip_path)
	with cd(app_dir):
		run('unzip -q -x %s -d %s' % (zip_path, sha1))
		run('rm -f %s' % zip_path)
	# make sure it can build
	with cd(code_path_remote):
		run("./scripts/build.sh")

	curr_dir = app_dir + '/current'
	if files.exists(curr_dir):
		# shut-down currently running instance
		sudo("/etc/init.d/fofou stop", pty=False)
		# rename old current as prev for easy rollback of bad deploy
		with cd(app_dir):
			run('rm -f prev')
			run('mv current prev')

	# make this version current
	with cd(app_dir):
		run("ln -s %s current" % sha1)

	if not files.exists("/etc/init.d/fofou"):
		sudo("ln -s /home/fofou/www/app/current/scripts/fofou.initd /etc/init.d/fofou")
		# make sure it runs on startup
		sudo("update-rc.d fofou defaults")

	# start it
	sudo("/etc/init.d/fofou start", pty=False)
	run("ps aux | grep fofou_app | grep -v grep")

	delete_old_deploys()
