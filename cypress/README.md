# Cypress Tests for Go RSS UI Application

This directory contains end-to-end tests for the Go RSS UI application using Cypress.

## Prerequisites

1. Node.js and npm installed
2. Go application running on `http://localhost:8080`
3. PostgreSQL database configured with the default admin user (username: `admin`, password: `admin`)

## Installation

```bash
npm install
```

## Running Tests

### Open Cypress Test Runner (Interactive)

```bash
npm run cypress:open
```

This will open the Cypress Test Runner where you can run individual tests interactively.

### Run All Tests (Headless)

```bash
npm run cypress:run
```

This will run all tests in headless mode and output results to the terminal.

### Run Tests with Custom Configuration

```bash
npm test
```

## Test Structure

- **`cypress/e2e/home.cy.js`** - Tests for the home page functionality
- **`cypress/e2e/auth.cy.js`** - Tests for authentication (login, logout, redirects)
- **`cypress/e2e/admin.cy.js`** - Tests for the admin panel functionality
- **`cypress/e2e/integration.cy.js`** - End-to-end integration tests covering full user flows

## Custom Commands

The tests use custom Cypress commands defined in `cypress/support/commands.js`:

- `cy.login(username, password)` - Logs in with the specified credentials
- `cy.logout()` - Logs out the current user
- `cy.shouldBeLoggedIn()` - Asserts that user is logged in and on admin page
- `cy.shouldBeLoggedOut()` - Asserts that user is logged out and on home page

## Test Data

The tests expect:
- Default admin user with username: `admin` and password: `admin`
- Application running on `http://localhost:8080`

## Configuration

Test configuration is defined in `cypress.config.js`. Key settings:
- Base URL: `http://localhost:8080`
- Viewport: 1280x720
- Timeouts: 10 seconds for commands and requests
