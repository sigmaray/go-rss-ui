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

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd go-rss-ui-2
   ```

2. Install Go dependencies:
   ```bash
   go mod download
   ```

3. Set up PostgreSQL database and configure the connection string:
   ```bash
   export DATABASE_URL="host=localhost user=youruser password=yourpass dbname=yourdb port=5432 sslmode=disable"
   ```

4. Run database migrations:
   ```bash
   go run . migrate
   ```

5. (Optional) Seed default admin user:
   ```bash
   go run . seed-users
   ```

6. Run the application:
   ```bash
   go run .
   ```

The application will start on `http://localhost:8082` (default port).

## Configuration

The application uses environment variables for configuration. Create a `.env` file or set the following variables:

- `DATABASE_URL` - PostgreSQL connection string
- `BACKGROUND_FETCH_ENABLED` - Enable/disable background feed fetching (default: true)
- `BACKGROUND_FETCH_INTERVAL` - Interval in seconds for background fetching (default: 3600)
- `CYPRESS` - Enable Cypress mode for testing tools (default: false)

## Default Credentials

When seeding users, a default admin user is created:
- **Username**: `admin`
- **Password**: `password` (or `admin` depending on seed command)

## CLI Commands

The application supports several CLI commands:

- `go run . migrate` - Run database migrations (create/update tables)
- `go run . seed-users` - Create default admin user
- `go run . seed-feeds` - Create default RSS feeds
- `go run . fetch-feeds` - Fetch and process all RSS feeds (creates/updates items)
- `go run . execute-sql "SELECT * FROM feeds"` - Execute SQL query (provide query as argument)
- `go run . execute-sql` - Execute SQL query interactively (reads from stdin)
- `go run . clear-users` - Clear all users from database
- `go run . create-db` - Create the application database
- `go run . drop-db` - Drop the application database

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
- `GET /tools` - Tools page (only when `CYPRESS=true`)
- `POST /tools/clear-all-tables` - Clear all database tables
- `POST /tools/clear-table` - Clear a specific table (requires `name` parameter: users, feeds, or items)
- `POST /tools/seed-users` - Seed users
- `POST /tools/seed-feeds` - Seed feeds
- `POST /tools/migrate` - Run migrations
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

2. Set `CYPRESS=true` environment variable:
   ```bash
   export CYPRESS=true
   ```

3. Make sure the Go application is running on `http://localhost:8082`

4. Run Cypress tests:
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

## Project Structure

```
go-rss-ui-2/
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

The application uses Gorm's AutoMigrate feature, which automatically creates/updates database tables based on the model definitions. Run migrations with:

```bash
go run . migrate
```

### Key Features Implementation

- **Cascade Deletion**: Implemented at database level using GORM constraints (`constraint:OnDelete:CASCADE`)
- **Username Uniqueness**: Enforced at both application and database levels
- **In-Memory Logging**: Thread-safe log storage with automatic size management
- **Background Fetching**: Configurable worker pool with concurrent processing
- **Pagination**: Implemented for users, feeds, and items using the paginate library

## License

This project is open source and available under the MIT License.
