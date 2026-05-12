kindle_cli/
├── main.go
├── go.mod
├── internal/
│   ├── tui/
│   │   ├── app.go              # Root Bubble Tea Model (holds screen stack)
│   │   ├── screens/
│   │   │   ├── search.go       # Manga search screen
│   │   │   ├── volumes.go      # Volume/chapter selection (multi-select list)
│   │   │   ├── confirm.go      # Confirmation before download
│   │   │   ├── download.go     # Download progress with progress bar
│   │   │   ├── settings.go     # Kindle email, API key, SMTP config
│   │   │   └── menu.go         # Main menu / home
│   │   └── components/
│   │       ├── list.go         # Reusable selectable list
│   │       ├── textinput.go    # Reusable text input
│   │       └── statusbar.go    # Status bar with help hints
│   ├── mangadex/
│   │   ├── client.go           # HTTP client with rate limiting
│   │   ├── search.go           # Search & feed endpoints
│   │   └── download.go         # At-home + image download + concurrency
│   ├── epub/
│   │   └── generator.go        # Image collection → EPUB per volume
│   ├── email/
│   │   └── kindle.go           # SMTP send with EPUB attachments
│   ├── config/
│   │   └── config.go           # Read/write settings file (JSON, ~/.kindle_cli.json)
│   └── cleanup/
│       └── cleanup.go          # Remove temp dirs after send
└── pkg/
    └── screenstack/
        └── stack.go            # Generic screen stack for TUI navigation