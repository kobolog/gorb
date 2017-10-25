#!/usr/bin/env python
#
# Usage:
#   ./autocompile.py path ext1,ext2,extn cmd
#
# Blocks monitoring |path| and its subdirectories for modifications on
# files ending with suffix |extk|. Run |cmd| each time a modification
# is detected. |cmd| is optional and defaults to 'make'.
#
# Example:
#   ./autocompile.py /my-latex-document-dir .tex,.bib "make pdf"
#
# Dependencies:
#   Linux, Python 2.6, Pyinotify
#
import subprocess
import sys
import pyinotify
import datetime

class OnWriteHandler(pyinotify.ProcessEvent):
    def my_init(self, cwd, cmd, extension):
        self.cwd = cwd
        self.cmd = cmd
        self.extension = extension

    def _run_cmd(self):
        self.date_start = datetime.datetime.now()
        print('==> Modification detected - %s' % self.date_start)
        subprocess.call(self.cmd.split(' '), cwd=self.cwd)
        self.date_end = datetime.datetime.now()
        print('==> Autocompile done - %s' % self.date_end)

    def process_IN_MODIFY(self, event):
        if all(not event.pathname.endswith(ext) for ext in self.extension):
            return
        self._run_cmd()

def auto_compile(path, extension, cmd):
    wm = pyinotify.WatchManager()
    handler = OnWriteHandler(cwd=path, cmd=cmd, extension=extension)
    notifier = pyinotify.Notifier(wm, default_proc_fun=handler)
    wm.add_watch(path, pyinotify.ALL_EVENTS, rec=True, auto_add=True)
    print('==> Start monitoring %s (type c^c to exit)' % path)
    notifier.loop()

if __name__ == '__main__':
    if len(sys.argv) < 2:
        print >> sys.stderr, "Command line error: missing argument(s)."
        sys.exit(1)

    # Required arguments
    path = sys.argv[1]
    extension = sys.argv[2]

    # Optional argument
    cmd = 'make'
    if len(sys.argv) == 4:
        cmd = sys.argv[3]

    # Blocks monitoring
    auto_compile(path, extension, cmd)
