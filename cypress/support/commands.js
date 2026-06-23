// ***********************************************
// This example commands.js shows you how to
// create various custom commands and overwrite
// existing commands.
//
// For more comprehensive examples of custom
// commands please read more here:
// https://on.cypress.io/custom-commands
// ***********************************************


// Seed default users (admin) via test tools endpoint
Cypress.Commands.add('seedUsers', () => {
  cy.request({
    method: 'POST',
    url: '/tools/seed-users',
    followRedirect: false,
    failOnStatusCode: false
  }).then((response) => {
    expect([200, 302]).to.include(response.status)
  })
})

// Clear all tables via test tools endpoint
Cypress.Commands.add('clearAllTables', () => {
  cy.request({
    method: 'POST',
    url: '/tools/clear-all-tables',
    followRedirect: false,
    failOnStatusCode: false
  }).then((response) => {
    expect([200, 302]).to.include(response.status)
  })
})

Cypress.Commands.add('clearTable', (tableName) => {
  cy.request({
    method: 'POST',
    url: '/tools/clear-table',
    body: 'name=' + tableName,
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded'
    },
    followRedirect: false,
    failOnStatusCode: false
  }).then((response) => {
    expect([200, 302]).to.include(response.status)
  })
})

// Log in with cy.session — caches session within the current spec file only
Cypress.Commands.add('loginWithSession', (username = 'admin', password = 'password') => {
  cy.session([Cypress.spec.relative, username, password], () => {
    cy.visit('/login')
    cy.get('input[name="username"]').should('be.visible').type(username)
    cy.get('input[name="password"]').should('be.visible').type(password)
    cy.get('button[type="submit"]').should('be.visible').click()
    cy.url({ timeout: 10000 }).should('include', '/admin/users')
    cy.get('nav.navbar').should('be.visible')
  }, {
    cacheAcrossSpecs: false,
    validate() {
      cy.request({
        url: '/admin/users',
        followRedirect: false,
        failOnStatusCode: false,
      }).its('status').should('eq', 200)
    },
  })

  cy.visit('/admin/users')
  cy.url().should('include', '/admin')
})

// Custom command to login
Cypress.Commands.add('login', (username = 'admin', password = 'password') => {
  cy.visit('/login')
  cy.get('input[name="username"]').type(username)
  cy.get('input[name="password"]').type(password)
  cy.get('button[type="submit"]').click()
  cy.url().should('include', '/admin/users')
})

// Custom command to logout
Cypress.Commands.add('logout', () => {
  cy.get('form[action="/logout"] button[type="submit"]').click()
  cy.url().should('eq', Cypress.config('baseUrl') + '/')
})

// Custom command to check if user is logged in
Cypress.Commands.add('shouldBeLoggedIn', () => {
  cy.url().should('include', '/admin')
})

// Custom command to check if user is logged out
Cypress.Commands.add('shouldBeLoggedOut', () => {
  cy.url().should('eq', Cypress.config('baseUrl') + '/')
})

// Auto-accept or reject window.confirm dialogs (uses Cypress window:confirm event)
Cypress.Commands.add('stubConfirm', (accept = true) => {
  cy.on('window:confirm', () => accept)
})
