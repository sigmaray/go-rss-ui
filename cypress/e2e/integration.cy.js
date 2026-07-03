describe('Full Application Flow', () => {
  before(() => {
    cy.clearAllTables()
    cy.seedUsers()
  })

  it('should complete full user journey: admin -> login -> admin -> logout', () => {
    cy.visit('/admin')

    // Should redirect to login
    cy.url().should('include', '/login')
    cy.get('h1').contains('Login').should('be.visible')

    // Login with admin credentials
    cy.get('input[name="username"]').type('admin')
    cy.get('input[name="password"]').type('password')
    cy.get('button[type="submit"]').click()

    // Should be redirected to admin panel
    cy.url().should('include', '/admin')
    cy.get('h1').contains('User Management').should('be.visible')

    // Verify admin user is listed
    cy.get('tbody tr').should('have.length.at.least', 1)
    cy.get('tbody tr').contains('td', 'admin')

    // Logout
    cy.get('form[action="/logout"] button[type="submit"]').click()

    // Should be back to home page
    cy.url().should('eq', Cypress.config('baseUrl') + '/')
    cy.contains('h1', 'RSS Feeds').should('be.visible')
    cy.get('nav.navbar').should('not.exist')
  })

  it('should handle failed login and retry successfully', () => {
    // Try to access admin without login
    cy.visit('/admin')
    cy.url().should('include', '/login')

    // Try wrong credentials
    cy.get('input[name="username"]').type('wrong')
    cy.get('input[name="password"]').type('wrong')
    cy.get('button[type="submit"]').click()
    cy.contains('Invalid credentials').should('be.visible')
    cy.url().should('include', '/login')

    // Try correct credentials
    cy.get('input[name="username"]').clear().type('admin')
    cy.get('input[name="password"]').clear().type('password')
    cy.get('button[type="submit"]').click()

    // Should login successfully
    cy.url().should('include', '/admin')
    cy.get('h1').contains('User Management').should('be.visible')
  })
})
