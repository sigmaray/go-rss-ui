# Cypress Tests for Go RSS UI Application

End-to-end tests for the Go RSS UI application using Cypress.

## Prerequisites

1. Node.js and npm installed
2. PostgreSQL database available (test database recommended, e.g. `go_rss_ui_test`)
3. Go application running on `http://localhost:8083` with Cypress mode enabled (see below)

## Starting the Application for Tests

Cypress tests call `/tools/*` endpoints (`clear-all-tables`, `seed-users`, `clear-table`, etc.). Those routes are only available when Cypress mode is enabled:

```bash
RSS_PORT=8083 RSS_DB_NAME=go_rss_ui_test RSS_CYPRESS=1 go run . server

# Or with hot reload:
RSS_CYPRESS=true RSS_PORT=8083 RSS_DB_NAME=go_rss_ui_test air
```

Set `RSS_CYPRESS=1` or `RSS_CYPRESS=true` (also accepts `yes` / `on`). Without this, tool requests return **403** and database setup commands in tests will fail.

## Installation

```bash
npm install
```

## Running Tests

### Open Cypress Test Runner (Interactive)

```bash
npm run cypress:open
```

### Run All Tests (Headless)

```bash
npm run cypress:run
# alias:
npm test
```

### Run Tests with Browser Visible

```bash
npm run cypress:run:headed
# alias:
npm test:headed
```

## Test Structure

| Spec | Coverage |
|------|----------|
| `cypress/e2e/home.cy.js` | Home page (title, login link) — no DB setup |
| `cypress/e2e/auth.cy.js` | Authentication (login, logout, redirects, invalid credentials) |
| `cypress/e2e/admin.cy.js` | Admin panel (users table, logout button) |
| `cypress/e2e/feeds.cy.js` | Feed CRUD, validation, bulk delete |
| `cypress/e2e/items.cy.js` | Items list/detail, empty state, bulk delete |
| `cypress/e2e/user_management.cy.js` | User CRUD, validation, uniqueness |
| `cypress/e2e/test_feeds.cy.js` | Fetching items from test feeds, fetch error handling |
| `cypress/e2e/integration.cy.js` | Full login/logout journey, failed login retry |

Most specs reset the database once per file in a `before()` hook:

```javascript
before(() => {
  cy.clearAllTables()
  cy.seedUsers()
})
```

Specs that require an authenticated session also call `cy.loginWithSession()` in `beforeEach()`.

## Custom Commands

Defined in `cypress/support/commands.js`:

| Command | Description |
|---------|-------------|
| `cy.clearAllTables()` | Clear all tables via `POST /tools/clear-all-tables` |
| `cy.seedUsers()` | Create default admin user via `POST /tools/seed-users` |
| `cy.clearTable(tableName)` | Clear one table (`users`, `feeds`, or `items`) via `POST /tools/clear-table` |
| `cy.loginWithSession(username, password)` | Log in with `cy.session` (default: `admin` / `password`); cached per spec file |
| `cy.login(username, password)` | Log in without session caching (default: `admin` / `password`) |
| `cy.logout()` | Log out the current user |
| `cy.shouldBeLoggedIn()` | Assert URL includes `/admin` |
| `cy.shouldBeLoggedOut()` | Assert URL is the home page |
| `cy.stubConfirm(accept)` | Stub `window.confirm` dialogs (default: accept) |

Tool commands accept HTTP `200` or `302` responses (the Go handlers may redirect to `/tools`).

## Test Data

- Default admin user: username `admin`, password `password` (created by `seed-users` / `cy.seedUsers()`)
- Application base URL: `http://localhost:8083` (override with `CYPRESS_BASE_URL` if needed)
- Application must run with `RSS_CYPRESS=1`

## Configuration

Defined in `cypress.config.js`:

- Base URL: `http://localhost:8083`
- Viewport: 1280×720
- Timeouts: 10 s (commands), 15 s (requests/responses)
- Retries: 2 in run mode, 0 in open mode
- Video recording: disabled
- Screenshots on failure: enabled

Support files are loaded from `cypress/support/e2e.js`, which imports `commands.js`.
