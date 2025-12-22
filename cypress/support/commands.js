// ***********************************************
// This example commands.js shows you how to
// create various custom commands and overwrite
// existing commands.
//
// For more comprehensive examples of custom
// commands please read more here:
// https://on.cypress.io/custom-commands
// ***********************************************

// Custom command to login
Cypress.Commands.add('login', (username = 'admin', password = 'password') => {
  cy.visit('/login')
  cy.get('input[name="username"]').type(username)
  cy.get('input[name="password"]').type(password)
  cy.get('input[type="submit"]').click()
  cy.url().should('include', '/admin')
})

// Custom command to logout
Cypress.Commands.add('logout', () => {
  cy.get('form[action="/logout"] input[type="submit"]').click()
  cy.url().should('eq', 'http://localhost:8080/')
})

// Custom command to check if user is logged in
Cypress.Commands.add('shouldBeLoggedIn', () => {
  cy.url().should('include', '/admin')
})

// Custom command to check if user is logged out
Cypress.Commands.add('shouldBeLoggedOut', () => {
  cy.url().should('eq', 'http://localhost:8080/')
})
