describe('Admin Panel', () => {
  beforeEach(() => {
    cy.clearUsersLoginRememberSession()
  })

  it('should display admin panel title', () => {
    cy.visit('/admin')
    cy.get('h1').contains('User Management').should('be.visible')
  })

  it('should display users section', () => {
    cy.visit('/admin')
    // Check for users table or users content
    cy.get('table').should('be.visible')
    cy.get('thead tr').should('contain', 'Username')
  })

  it('should display users table with headers', () => {
    cy.visit('/admin')
    cy.get('table').should('be.visible')
    cy.get('thead tr th').should('have.length', 4)
    cy.get('thead tr th').first().should('contain', 'ID')
    cy.get('thead tr th').eq(1).should('contain', 'Username')
    cy.get('thead tr th').eq(2).should('contain', 'Created At')
    cy.get('thead tr th').eq(3).should('contain', 'Actions')
  })

  it('should have at least one user (admin)', () => {
    cy.visit('/admin')
    cy.get('tbody tr').should('have.length.at.least', 1)
    cy.get('tbody tr').contains('td', 'admin')
  })

  it('should have logout button', () => {
    cy.visit('/admin')
    cy.get('form[action="/logout"]').should('be.visible')
    cy.get('form[action="/logout"] button[type="submit"]').should('be.visible').should('contain', 'Logout')
  })
})
