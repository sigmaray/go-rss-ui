describe('Admin Panel', () => {
  beforeEach(() => {
    cy.login()
  })

  it('should display admin panel title', () => {
    cy.visit('/admin')
    cy.get('h1').contains('Admin Panel').should('be.visible')
  })

  it('should display users section', () => {
    cy.visit('/admin')
    cy.get('h2').contains('Users').should('be.visible')
  })

  it('should display users table with headers', () => {
    cy.visit('/admin')
    cy.get('table').should('be.visible')
    cy.get('thead tr th').should('have.length', 3)
    cy.get('thead tr th').first().should('contain', 'ID')
    cy.get('thead tr th').eq(1).should('contain', 'Username')
    cy.get('thead tr th').eq(2).should('contain', 'Created At')
  })

  it('should have at least one user (admin)', () => {
    cy.visit('/admin')
    cy.get('tbody tr').should('have.length.at.least', 1)
    cy.get('tbody tr').first().find('td').eq(1).should('contain', 'admin')
  })

  it('should have logout button', () => {
    cy.visit('/admin')
    cy.get('form[action="/logout"]').should('be.visible')
    cy.get('form[action="/logout"] input[type="submit"]').should('have.value', 'Logout')
  })
})
