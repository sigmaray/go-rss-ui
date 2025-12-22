# Go RSS UI Application

A simple web application built with Go, Gin, Gorm, and PostgreSQL that provides user authentication and an admin panel to view users.

## Features

- **Home Page**: Displays the application title and link to admin panel
- **Authentication**: Session-based login system with automatic redirects
- **Admin Panel**: Protected area showing a list of all users in the database
- **User Management**: Automatic seeding of default admin user

## Tech Stack

- **Backend**: Go with Gin web framework
- **Database**: PostgreSQL with Gorm ORM
- **Authentication**: Session-based with Gin sessions
- **Templates**: HTML templates for server-side rendering
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

4. Run the application:
   ```bash
   go run .
   ```

The application will start on `http://localhost:8080`.

## Default Credentials

When the application starts for the first time, it automatically creates a default admin user:
- **Username**: `admin`
- **Password**: `admin`

## API Endpoints

- `GET /` - Home page
- `GET /login` - Login form
- `POST /login` - Process login
- `GET /admin` - Admin panel (requires authentication)
- `POST /logout` - Logout

## Testing

### End-to-End Tests with Cypress

1. Install Node.js dependencies:
   ```bash
   npm install
   ```

2. Make sure the Go application is running on `http://localhost:8080`

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
- Admin panel access and user listing
- Error handling for invalid credentials
- Complete user journey integration tests

## Project Structure

```
go-rss-ui-2/
├── main.go           # Application entry point and routes
├── database.go       # Database connection and setup
├── models.go         # Data models (User)
├── templates/        # HTML templates
│   ├── index.html    # Home page template
│   ├── login.html    # Login form template
│   └── admin.html    # Admin panel template
├── cypress/          # End-to-end tests
│   ├── e2e/         # Test files
│   ├── support/     # Custom commands and support files
│   └── README.md    # Testing documentation
├── package.json      # Node.js dependencies for testing
├── cypress.config.js # Cypress configuration
└── README.md         # This file
```

## Development

### Adding New Features

1. Update routes in `main.go`
2. Add new templates in `templates/` directory
3. Update models in `models.go` if needed
4. Add corresponding Cypress tests

### Database Migrations

The application uses Gorm's AutoMigrate feature, which automatically creates/updates database tables based on the model definitions.

## License

This project is open source and available under the MIT License.
