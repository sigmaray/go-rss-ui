# Go RSS UI Application

A comprehensive RSS feed management web application built with Go, Gin, Gorm, and PostgreSQL. The application provides user authentication, RSS feed management, automatic feed fetching, and detailed logging.

## Features

### Core Features
- **Home Page**: Displays the application title and navigation
- **Authentication**: Session-based login system with automatic redirects
- **User Management**: 
  - Create, edit, and delete users
  - Username uniqueness validation
  - Password hashing with bcrypt
  - Pagination support

### RSS Feed Management
- **Feed Management**: 
  - Add, view, and delete RSS feeds
  - Automatic feed fetching with background worker
  - Feed status tracking (last successful fetch, errors)
  - Bulk operations (delete all feeds, seed default feeds)
- **Item Management**:
  - View RSS items with pagination
  - Automatic item creation and updates
  - Detailed item view with full content
  - Manual feed fetching
  - Bulk delete operations
- **Cascade Deletion**: When a feed is deleted, all associated items are automatically deleted (database-level cascade)

### Logging
- **In-Memory Logging**: 
  - Real-time feed fetch logs
  - Success and error tracking
  - Maximum 1000 log entries (oldest entries automatically removed)
  - Detailed information: created/updated item counts, error messages
  - Accessible via `/logs` route (authenticated users only)

### Background Processing
- **Automatic Feed Fetching**: 
  - Configurable background worker
  - Periodic feed updates
  - Concurrent processing (up to 10 workers)
  - Error handling and retry logic

## Tech Stack

- **Backend**: Go with Gin web framework
- **Database**: PostgreSQL with Gorm ORM
- **Authentication**: Session-based with Gin sessions
- **Templates**: HTML templates for server-side rendering
- **RSS Parsing**: gofeed library
- **Testing**: Cypress for end-to-end testing

## Prerequisites

- Go 1.19 or later
- PostgreSQL database
- Node.js and npm (for testing)

## Installation

### Option 1: Using Docker Compose (Recommended)

The easiest way to run the application is using Docker Compose:

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd go-rss-ui
   ```

2. Start the application and PostgreSQL database:
   ```bash
   docker-compose up -d
   ```

3. Run database migrations:
   ```bash
   docker-compose exec app ./go-rss-ui migrate
   ```

4. (Optional) Seed default admin user:
   ```bash
   docker-compose exec app ./go-rss-ui users-seed
   ```

The application will be available at `http://localhost:8082`.

To stop the application:
```bash
docker-compose down
```

To view logs:
```bash
docker-compose logs -f app
```

### Option 2: Manual Installation

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd go-rss-ui
   ```

2. Install Go dependencies:
   ```bash
   go mod download
   ```

3. Set up PostgreSQL database and configure the connection string:
   ```bash
   export GO_RSS_UI_DATABASE_URL="host=localhost user=youruser password=yourpass dbname=yourdb port=5432 sslmode=disable"
   ```
   Or use individual variables:
   ```bash
   export GO_RSS_UI_DB_HOST=localhost
   export GO_RSS_UI_DB_USER=youruser
   export GO_RSS_UI_DB_PASSWORD=yourpass
   export GO_RSS_UI_DB_NAME=yourdb
   export GO_RSS_UI_DB_PORT=5432
   export GO_RSS_UI_DB_SSLMODE=disable
   export GO_RSS_UI_DB_TIMEZONE=UTC
   export GO_RSS_UI_SESSION_SECRET=replace-with-a-long-random-secret-at-least-32-characters
   ```

4. Run database migrations:
   ```bash
   go run . migrate
   ```

5. (Optional) Seed default admin user:
   ```bash
   go run . users-seed
   ```

6. Run the application:
   ```bash
   go run . server
   ```

The application will start on `http://localhost:8082` (default port).

### Docker Build

To build the Docker image manually:

```bash
docker build -t go-rss-ui:latest .
```

To run the container:

```bash
docker run -d \
  --name go-rss-ui-app \
  -p 8082:8082 \
  -e GO_RSS_UI_DB_HOST=postgres \
  -e GO_RSS_UI_DB_USER=postgres \
  -e GO_RSS_UI_DB_PASSWORD=postgres \
  -e GO_RSS_UI_DB_NAME=go_rss_ui \
  -e GO_RSS_UI_DB_PORT=5432 \
  -e GO_RSS_UI_SESSION_SECRET=replace-with-a-long-random-secret-at-least-32-characters \
  go-rss-ui:latest
```

For Docker Compose, pass `GO_RSS_UI_SESSION_SECRET` from the environment or an untracked local `.env` file instead of committing a real production secret into the repository.

On Ubuntu, you can generate and provide the secret in either of these ways:

```bash
# Option 1: current shell session only
export GO_RSS_UI_SESSION_SECRET="$(openssl rand -base64 48)"
docker compose up -d
```

```bash
# Option 2: local .env file next to docker-compose.yml
printf 'GO_RSS_UI_SESSION_SECRET=%s\n' "$(openssl rand -base64 48)" >> .env
docker compose up -d
```

If you use the `.env` file approach, keep that file out of version control and store the real production value only on the target server or in your secret manager.

## Configuration

The application uses environment variables for configuration. All variables use the `GO_RSS_UI_` prefix. Create a `.env` file or set the following variables:

### Database Configuration
- `GO_RSS_UI_DATABASE_URL` - Complete PostgreSQL connection string (takes precedence over individual variables)
- `GO_RSS_UI_DB_HOST` - PostgreSQL database host (default: localhost)
- `GO_RSS_UI_DB_USER` - PostgreSQL database user (default: postgres)
- `GO_RSS_UI_DB_PASSWORD` - PostgreSQL database password (default: postgres)
- `GO_RSS_UI_DB_NAME` - PostgreSQL database name (default: go_rss_ui)
- `GO_RSS_UI_DB_PORT` - PostgreSQL database port (default: 5432)
- `GO_RSS_UI_DB_SSLMODE` - PostgreSQL SSL mode (default: disable)
- `GO_RSS_UI_DB_TIMEZONE` - PostgreSQL timezone (default: Asia/Shanghai)

### Redis Configuration
- `GO_RSS_UI_REDIS_HOST` - Redis host (default: localhost)
- `GO_RSS_UI_REDIS_PORT` - Redis port (default: 6379)
- `GO_RSS_UI_REDIS_PASSWORD` - Redis password (default: empty)

### Server Configuration
- `GO_RSS_UI_PORT` - Server port (default: 8082)
- `GO_RSS_UI_ENV` - Environment name; set to `production` in production deployments

### Session Configuration
- `GO_RSS_UI_SESSION_SECRET` - Session signing secret; required in production and should be at least 32 characters long
- `GO_RSS_UI_SESSION_SECURE` - Explicit override for the session cookie `Secure` flag; when omitted, secure cookies are enabled automatically in production

### Background Feed Fetching
- `GO_RSS_UI_BACKGROUND_FETCH_ENABLED` - Enable/disable background feed fetching (default: true)
- `GO_RSS_UI_BACKGROUND_FETCH_INTERVAL` - Interval in seconds for background fetching (default: 60)

### Testing
- `GO_RSS_UI_CYPRESS` - Enable Cypress mode for testing tools (default: false)

## Default Credentials

When seeding users, a default admin user is created:
- **Username**: `admin`
- **Password**: `password` (or `admin` depending on seed command)

## CLI Commands

The application supports several CLI commands:

- `go run . server` (or `go run . s`) - Start the web server
- `go run . migrate` - Run pending database migrations (goose)
- `go run . migrate-status` - Show database migration status
- `go run . migrate-down` - Roll back the last database migration
- `go run . automigrate` - Create tables in database using AutoMigrate
- `go run . users-seed` - Create default admin user
- `go run . feeds-seed` - Create default RSS feeds
- `go run . feeds-fetch` - Fetch and process all RSS feeds (creates/updates items)
- `go run . execute-sql "SELECT * FROM feeds"` - Execute SQL query (provide query as argument)
- `go run . execute-sql` - Execute SQL query interactively (reads from stdin)
- `go run . users-clear` - Clear all users from database
- `go run . db-create` - Create the application database
- `go run . db-drop` - Drop the application database

## API Endpoints

### Public Routes
- `GET /` - Home page
- `GET /login` - Login form
- `POST /login` - Process login
- `POST /logout` - Logout

### Protected Routes (Require Authentication)

#### User Management
- `GET /admin/users` - List all users (with pagination)
- `GET /admin/users/new` - Show create user form
- `POST /admin/users` - Create new user
- `GET /admin/users/:id/edit` - Show edit user form
- `POST /admin/users/:id/edit` - Update user
- `POST /admin/users/:id/delete` - Delete user

#### Feed Management
- `GET /admin/feeds` - List all feeds (with pagination)
- `GET /admin/feeds/new` - Show create feed form
- `POST /admin/feeds` - Create new feed
- `POST /admin/feeds/:id/delete` - Delete feed (cascade deletes items)
- `POST /admin/feeds/delete-all` - Delete all feeds
- `POST /admin/feeds/seed` - Seed default feeds

#### Item Management
- `GET /admin/items` - List all items (with pagination)
- `GET /admin/items/:id` - View item details
- `POST /admin/items/fetch` - Manually fetch all feeds
- `POST /admin/items/delete-all` - Delete all items

#### Logs
- `GET /logs` - View feed fetch logs (in-memory, max 1000 entries)

#### Tools (Cypress Mode Only)
- `GET /tools` - Tools page (only when `GO_RSS_UI_CYPRESS=true`)
- `POST /tools/clear-all-tables` - Clear all database tables
- `POST /tools/clear-table` - Clear a specific table (requires `name` parameter: users, feeds, or items)
- `POST /tools/seed-users` - Seed users
- `POST /tools/seed-feeds` - Seed feeds
- `POST /tools/automigrate` - Run migrations (AutoMigrate)
- `POST /tools/migrate` - Run migrations (goose)
- `POST /tools/drop-db` - Drop database
- `POST /tools/create-db` - Create database
- `POST /tools/execute-sql` - Execute SQL queries

## Database Models

### User
- `ID` - Primary key
- `Username` - Unique username (enforced at database level)
- `Password` - Bcrypt hashed password
- `CreatedAt`, `UpdatedAt`, `DeletedAt` - Timestamps

### Feed
- `ID` - Primary key
- `URL` - Unique feed URL
- `Title` - Feed title
- `Description` - Feed description
- `LastSuccessfullyFetchedAt` - Timestamp of last successful fetch
- `LastError` - Last error message
- `LastErrorAt` - Timestamp of last error
- `Items` - Related items (cascade delete)

### Item
- `ID` - Primary key
- `FeedID` - Foreign key to Feed (cascade delete)
- `Title` - Item title
- `Link` - Item link
- `Description` - Item description
- `Content` - Item content
- `Author` - Item author
- `PublishedAt` - Publication date
- `GUID` - Unique identifier from feed
- `Feed` - Related feed

## Testing

### End-to-End Tests with Cypress

1. Install Node.js dependencies:
   ```bash
   npm install
   ```

2. Start the web server for Cypress tests with the required environment variables:
   ```bash
   # Using air (for hot reload during development)
   GO_RSS_UI_PORT=8083 GO_RSS_UI_DB_NAME=go_rss_ui_test GO_RSS_UI_CYPRESS=1 air

   # Or using go run
   GO_RSS_UI_PORT=8083 GO_RSS_UI_DB_NAME=go_rss_ui_test GO_RSS_UI_CYPRESS=1 go run . server
   ```

   The server will start on `http://localhost:8083` (as configured in `cypress.config.js`).

3. Run Cypress tests:
   ```bash
   # Interactive mode
   npm run cypress:open

   # Headless mode
   npm run cypress:run
   ```

### Test Coverage

The Cypress tests cover:
- Home page functionality
- Authentication flow (login/logout)
- User management (create, edit, delete, username uniqueness)
- Feed management (create, delete, bulk operations)
- Item management (view, fetch, delete)
- Logs viewing
- Error handling
- Complete user journey integration tests

## Example `.env`

See [`.env.example`](/home/foobar/r/sandbox/go-rss-ui/.env.example) for the current variable names and defaults. The application expects `GO_RSS_UI_*` variables such as `GO_RSS_UI_DB_HOST`, `GO_RSS_UI_DATABASE_URL`, `GO_RSS_UI_CYPRESS`, and `GO_RSS_UI_PORT`.

## Project Structure

```
go-rss-ui/
├── main.go              # Application entry point, routes, and handlers
├── database.go          # Database connection and setup
├── models.go            # Data models (User, Feed, Item)
├── commands.go          # CLI commands implementation
├── config.go            # Configuration management
├── templates/           # HTML templates
│   ├── layouts/         # Layout templates
│   │   └── layout.html  # Main layout
│   ├── partials/        # Partial templates
│   │   └── pagination.html
│   ├── index.html       # Home page
│   ├── login.html       # Login form
│   ├── users.html       # User list
│   ├── create_user.html # Create user form
│   ├── edit_user.html   # Edit user form
│   ├── feeds.html       # Feed list
│   ├── create_feed.html # Create feed form
│   ├── items.html       # Item list
│   ├── item.html        # Item details
│   ├── logs.html        # Logs view
│   ├── admin.html       # Admin panel
│   └── tools.html       # Tools page (Cypress mode)
├── static/              # Static files
│   └── css/
│       └── styles.css   # Stylesheet
├── test_feeds/          # Test RSS feeds
├── cypress/             # End-to-end tests
│   ├── e2e/            # Test files
│   ├── support/        # Custom commands and support files
│   └── README.md       # Testing documentation
├── package.json         # Node.js dependencies for testing
├── cypress.config.js    # Cypress configuration
└── README.md            # This file
```

## Development

### Adding New Features

1. Update routes in `main.go`
2. Add new templates in `templates/` directory
3. Update models in `models.go` if needed
4. Add corresponding Cypress tests in `cypress/e2e/`

### Database Migrations

The application uses [goose](https://github.com/pressly/goose) for SQL migrations. Migration files live in `database/migrations/`.

```bash
go run . migrate
go run . migrate-status
go run . migrate-down
```

To add a new migration, create a file in `database/migrations/` with the next sequential number, for example `00002_add_column.sql`:

```sql
-- +goose Up
ALTER TABLE feeds ADD COLUMN example TEXT;

-- +goose Down
ALTER TABLE feeds DROP COLUMN example;
```

### Key Features Implementation

- **Cascade Deletion**: Implemented at database level using GORM constraints (`constraint:OnDelete:CASCADE`)
- **Username Uniqueness**: Enforced at both application and database levels
- **In-Memory Logging**: Thread-safe log storage with automatic size management
- **Background Fetching**: Configurable worker pool with concurrent processing
- **Pagination**: Implemented for users, feeds, and items using the paginate library

## Screenshots

<img width="1306" height="1074" alt="image" src="https://github.com/user-attachments/assets/5166352d-f0f6-4524-b48b-c24babaaa4a4" />

<img width="1306" height="1074" alt="image" src="https://github.com/user-attachments/assets/7a71ae73-a971-40fd-9161-2ea64b2d07c7" />

<img width="1306" height="1074" alt="image" src="https://github.com/user-attachments/assets/3bcbf654-2c8e-428a-a5be-6589513fcaaa" />

<img width="1306" height="1130" alt="image" src="https://github.com/user-attachments/assets/eb9bff67-512f-4867-8605-8ce3891a1eae" />

<img width="1306" height="1130" alt="image" src="https://github.com/user-attachments/assets/cc29b8d6-cd7f-4526-8d9b-cf9c35bc4eb2" />

## License

This project is open source and available under the MIT License.
