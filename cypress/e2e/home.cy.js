describe('Home Page', () => {
  it('should display the home page with correct title and no navbar when logged out', () => {
    cy.visit('/')
    cy.contains('h1', 'RSS Feeds').should('be.visible')
    cy.get('nav.navbar').should('not.exist')
  })
})
