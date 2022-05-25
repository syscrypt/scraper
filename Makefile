SRC    = main.go
SRCDIR = ./cmds/scraper/
EXEC   = scraper
BINDIR = ./bin/
CC     = go
BLD    = build

all: build run

.PHONY: build run

build:
	@mkdir -p $(BINDIR)
	$(CC) $(BLD) -o $(BINDIR)$(EXEC) $(SRCDIR)$(SRC)

run:
	@$(BINDIR)$(EXEC)
