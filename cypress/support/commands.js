// ***********************************************
// This example commands.js shows you how to
// create various custom commands and overwrite
// existing commands.
//
// For more comprehensive examples of custom
// commands please read more here:
// https://on.cypress.io/custom-commands
// ***********************************************


// Custom command to setup database (clear and seed users)
// Note: This is now handled inside loginRememberSession for better session management
Cypress.Commands.add('setupDatabase', () => {
  // Clear all sessions to ensure fresh state
  cy.clearCookies()
  cy.clearLocalStorage()
  
  // Clear database - expects redirect (302) or success (200)
  cy.request({
    method: 'POST',
    url: '/tools/clear-database',
    followRedirect: false,
    failOnStatusCode: false
  }).then((response) => {
    // Accept both redirect (302) and success (200) status codes
    expect([200, 302]).to.include(response.status)
  })
  
  // Seed users - expects redirect (302) or success (200)
  cy.request({
    method: 'POST',
    url: '/tools/seed-users',
    followRedirect: false,
    failOnStatusCode: false
  }).then((response) => {
    // Accept both redirect (302) and success (200) status codes
    expect([200, 302]).to.include(response.status)
  })
})

// Custom command to login
Cypress.Commands.add('loginRememberSession', (username = 'admin', password = 'password') => {
  cy.session([username, password], () => {
    // Setup database before login to ensure user exists
    cy.request({
      method: 'POST',
      url: '/tools/clear-database',
      followRedirect: false,
      failOnStatusCode: false
    })
    cy.request({
      method: 'POST',
      url: '/tools/seed-users',
      followRedirect: false,
      failOnStatusCode: false
    })
    
    // Now login
    cy.visit('/login')
    cy.get('input[name="username"]').should('be.visible').type(username)
    cy.get('input[name="password"]').should('be.visible').type(password)
    cy.get('button[type="submit"]').should('be.visible').click()
    cy.url({ timeout: 10000 }).should('include', '/admin/users')
    // Verify we're logged in by checking for admin menu
    cy.get('nav.admin-menu').should('be.visible')
  }, {
    cacheAcrossSpecs: false
  });
  
  // After session is restored, visit a page to ensure session is active
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
  cy.url().should('eq', 'http://localhost:8082/')
})

// Custom command to check if user is logged in
Cypress.Commands.add('shouldBeLoggedIn', () => {
  cy.url().should('include', '/admin')
})

// Custom command to check if user is logged out
Cypress.Commands.add('shouldBeLoggedOut', () => {
  cy.url().should('eq', 'http://localhost:8082/')
})
