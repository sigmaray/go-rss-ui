describe('Home Page', () => {
  it('should display the home page with correct title and admin link', () => {
    cy.visit('/')
    cy.contains('h1', 'My RSS App').should('be.visible')
    cy.contains('a', 'Go to Admin').should('be.visible')
  })

  it('should have a link to admin page', () => {
    cy.visit('/')
    cy.get('a[href="/admin"]').should('exist')
  })
})
