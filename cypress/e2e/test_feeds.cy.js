describe('Test Feeds Fetch', () => {
  beforeEach(() => {
    cy.loginRememberSession()
    // Ensure we're logged in before each test
    cy.visit('/admin/feeds')
    cy.url().should('include', '/admin/feeds')
  })

  it('should fetch items from test feeds', () => {
    // Visit feeds page
    cy.visit('/admin/feeds')
    
    // Create first test feed using direct form submission
    cy.visit('/admin/feeds/new')
    cy.get('input[name="url"]').type('http://localhost:8082/test_feeds/test1.xml')
    cy.get('form[action="/admin/feeds"]').submit()
    cy.url({ timeout: 10000 }).should('include', '/admin/feeds')
    // Check for success message if present
    cy.get('body').then(($body) => {
      cy.get('.success').should('contain', 'Feed created successfully')
    })
    
    // Create second test feed
    cy.visit('/admin/feeds/new')
    cy.get('input[name="url"]').type('http://localhost:8082/test_feeds/test2.xml')
    cy.get('form[action="/admin/feeds"]').submit()
    cy.url({ timeout: 10000 }).should('include', '/admin/feeds')
    // Check for success message if present
    cy.get('body').then(($body) => {
      cy.get('.success').should('contain', 'Feed created successfully')
    })
    
    // Verify feeds are created - check for the full URL
    cy.visit('/admin/feeds')

    cy.get('body').then(($body) => {
        cy.get('tbody tr').should('contain', 'test_feeds')
    })
    
    // Navigate to items page
    cy.visit('/admin/items')
    
    // Click "Fetch Feed Items" button
    cy.get('form[action="/admin/items/fetch"]').first().submit()
    
    // Wait for redirect
    cy.url({ timeout: 10000 }).should('include', '/admin/items')
    
    // Wait for success message - it should indicate items were created
    cy.get('.success', { timeout: 10000 }).should('be.visible').should('contain', 'Fetched items')
    
    // The success message format is: "Fetched items: X created, Y updated"
    // Just verify it contains "Fetched items" which we already checked above
    
    // Verify that items exist in the table
    cy.get('tbody tr').should('have.length.at.least', 1)
    
    // Now search for test items - they should be in the database
    // Since items are ordered by newest first, test items should be on the first page
    // Let's check if any test items are visible
    cy.get('body').then(($body) => {
      const bodyText = $body.text()
      // At least one test item should be visible
      const hasTestItem = bodyText.includes('Test Item 1') || 
                         bodyText.includes('Test Item 2') || 
                         bodyText.includes('Test Item A') || 
                         bodyText.includes('Test Item B') || 
                         bodyText.includes('Test Item C')

        // At least one test item is visible, verify it
        cy.get('tbody tr').should('satisfy', ($rows) => {
          const rowText = Array.from($rows).map(row => row.textContent).join(' ')
          return rowText.includes('Test Item 1') || 
                 rowText.includes('Test Item 2') || 
                 rowText.includes('Test Item A') || 
                 rowText.includes('Test Item B') || 
                 rowText.includes('Test Item C')
        })
    })
  })
  
  // it('should not fetch test feeds in background', () => {
  //   // This test verifies that test feeds are excluded from background fetching
  //   // We'll create a test feed and verify it's not fetched automatically
    
  //   // Create a test feed
  //   cy.visit('/admin/feeds/new')
  //   cy.get('input[name="url"]').type('http://localhost:8082/test_feeds/test1.xml')
  //   cy.get('form[action="/admin/feeds"]').submit()
  //   cy.url({ timeout: 10000 }).should('include', '/admin/feeds')
    
  //   // Get initial item count
  //   cy.visit('/admin/items')
  //   cy.get('tbody tr').then(($rows) => {
  //     const initialCount = $rows.length
      
  //     // Wait a bit to ensure background fetcher would have run if it was going to
  //     cy.wait(5000)
      
  //     // Refresh items page
  //     cy.visit('/admin/items')
      
  //     // Item count should not have changed (test feeds are excluded from background fetch)
  //     cy.get('tbody tr').should('have.length', initialCount)
  //   })
  // })
})

