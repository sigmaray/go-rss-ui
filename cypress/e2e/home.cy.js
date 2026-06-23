describe('Home Page', () => {
  it('should display the home page with correct title and admin link', () => {
    cy.visit('/')
    cy.contains('h1', 'My RSS App').should('be.visible')
    cy.get('a[href="/login"]').should('be.visible').should('contain', 'Login')
  })
})
