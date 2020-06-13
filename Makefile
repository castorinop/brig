# RUN_PREFIX : what the prefix is when the software is run. usually the same as PREFIX
PREFIX?=/usr
INSTALLDIR?=$(DESTDIR)$(PREFIX)
DOCDIR?=$(INSTALLDIR)/share/doc/brig
RUN_PREFIX?=$(PREFIX)

build:
	go run mage.go

install:
	install -d $(INSTALLDIR)
	install -d $(INSTALLDIR)/bin
	install -d $(DOCDIR)
	install -m755 brig $(INSTALLDIR)/bin/brig
	install -m755 README.md $(DOCDIR)/README.md
	install -m755 autocomplete/bash_autocomplete $(DOCDIR)/bash_autocomplete
	install -m755 autocomplete/zsh_autocomplete $(DOCDIR)/zsh_autocomplete

