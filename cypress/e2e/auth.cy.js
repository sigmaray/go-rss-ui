describe('Authentication', () => {
  beforeEach(() => {
    // Clear cookies to ensure clean session state
    cy.clearCookies()
    cy.clearLocalStorage()
    // Setup database with admin user (done via cy.request since we're not logging in yet)
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
  })

  it('should redirect to login when accessing admin without authentication', () => {
    cy.visit('/admin')
    cy.url().should('include', '/login')
  })

  it('should display login form', () => {
    cy.visit('/login')
    cy.get('h1').contains('Login').should('be.visible')
    cy.get('input[name="username"]').should('be.visible')
    cy.get('input[name="password"]').should('be.visible')
    cy.get('button[type="submit"]').should('be.visible')
  })

  it('should login with correct credentials', () => {
    cy.login('admin', 'password')
    cy.shouldBeLoggedIn()
  })

  it('should show error message with incorrect credentials', () => {
    cy.visit('/login')
    cy.get('input[name="username"]').type('wronguser')
    cy.get('input[name="password"]').type('wrongpass')
    cy.get('button[type="submit"]').click()
    cy.contains('Invalid credentials').should('be.visible')
    cy.url().should('include', '/login')
  })

  it('should logout successfully', () => {
    cy.login()
    cy.shouldBeLoggedIn()
    cy.logout()
    cy.shouldBeLoggedOut()
  })

  it('should redirect to login after logout when accessing admin', () => {
    cy.login()
    cy.logout()
    cy.visit('/admin')
    cy.url().should('include', '/login')
  })
})
