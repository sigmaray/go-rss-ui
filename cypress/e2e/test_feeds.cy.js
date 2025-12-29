describe('Test Feeds Fetch', () => {
  beforeEach(() => {
    cy.clearUsersLoginRememberSession()
    cy.clearTable('feeds')
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
    
    cy.get('.success').should('contain', 'Feed created successfully')    
    
    // Create second test feed
    cy.visit('/admin/feeds/new')
    cy.get('input[name="url"]').type('http://localhost:8082/test_feeds/test2.xml')
    cy.get('form[action="/admin/feeds"]').submit()
    cy.url({ timeout: 10000 }).should('include', '/admin/feeds')
    
    cy.get('.success').should('contain', 'Feed created successfully')
    
    // Verify feeds are created - check for the full URL
    cy.visit('/admin/feeds')

    cy.get('table tbody tr').should('contain', '/test_feeds/')
    
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
    
    cy.get('table tr').should('contain','Test Item 1')
    cy.get('table tr').should('contain','Test Item 2')
    cy.get('table tr').should('contain','Test Item A')
    cy.get('table tr').should('contain','Test Item B')
    cy.get('table tr').should('contain','Test Item C')
    
    // Navigate to info page and verify statistics
    cy.visit('/info')
    cy.url().should('include', '/info')
    cy.get('h1').should('contain', 'System Information')
    cy.get('h2').should('contain', 'System Statistics')
    
    // Check Database Statistics section
    cy.get('h3').contains('Database Statistics').should('be.visible')
        
    // Verify Total Feeds count (should be 2)
    cy.contains('td', 'Total Feeds:').parent('tr').within(() => {
      cy.get('td').eq(1).should('contain', '2')
    })
    
    // Verify Total Items count (should be at least 5)
    cy.contains('td', 'Total Items:').parent('tr').within(() => {
      cy.get('td').eq(1).invoke('text').then((text) => {
        const itemsCount = parseInt(text.trim())
        expect(itemsCount).to.eq(5)
      })
    })
    
    // Check Feed Fetch Status section
    cy.get('h3').contains('Feed Fetch Status').should('be.visible')
    
    // Verify Last Successful Fetch exists and contains timestamp
    cy.contains('td', 'Last Successful Fetch:').parent('tr').within(() => {
      cy.get('td').eq(1).should('not.contain', 'Never')
      // Check that it contains a timestamp format (YYYY-MM-DD HH:MM:SS)
      cy.get('td').eq(1).invoke('text').then((text) => {
        expect(text).to.match(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/)
      })
      // Verify it contains one of the test feed URLs
      cy.get('td').eq(1).should('contain', '/test_feeds/')
    })
    
    // Verify Last Failed Fetch (should be "Never" if no errors, or contain timestamp if there were errors)
    cy.contains('td', 'Last Failed Fetch:').parent('tr').within(() => {
      cy.get('td').eq(1).then(($td) => {
        const text = $td.text()
        if (text.includes('Never')) {
          // No errors, which is fine
          cy.get('td').eq(1).should('contain', 'Never')
        } else {
          // There were errors, verify format
          expect(text).to.match(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/)
        }
      })
    })
    
    // Verify Environment Variables section exists
    cy.get('h3').contains('Environment Variables').should('be.visible')
    cy.get('table').contains('th', 'Variable Name').should('be.visible')
    cy.get('table').contains('th', 'Value').should('be.visible')
    cy.get('table').contains('th', 'Description').should('be.visible')
  })
  
  it('should handle feed fetch errors (404 and 500) and display them in UI', () => {
    // Visit feeds page
    cy.visit('/admin/feeds')
    
    // Create feed with 404 error
    cy.visit('/admin/feeds/new')
    cy.get('input[name="url"]').type('http://localhost:8082/test_feeds/error404.xml')
    cy.get('form[action="/admin/feeds"]').submit()
    cy.url({ timeout: 10000 }).should('include', '/admin/feeds')
    cy.get('.success').should('contain', 'Feed created successfully')
    
    // Create feed with 500 error
    cy.visit('/admin/feeds/new')
    cy.get('input[name="url"]').type('http://localhost:8082/test_feeds/error500.xml')
    cy.get('form[action="/admin/feeds"]').submit()
    cy.url({ timeout: 10000 }).should('include', '/admin/feeds')
    cy.get('.success').should('contain', 'Feed created successfully')
    
    // Verify feeds are created
    cy.visit('/admin/feeds')
    cy.get('table tbody tr').should('have.length.at.least', 2)
    
    // Find the row with error404.xml feed and click Fetch button
    cy.contains('table tbody tr', 'error404.xml').within(() => {
      cy.get('form[action*="/fetch"] button[type="submit"]').click()
    })
    
    // Wait for redirect back to feeds page
    cy.url({ timeout: 10000 }).should('include', '/admin/feeds')
    
    // Verify error is displayed for 404 feed
    cy.contains('table tbody tr', 'error404.xml').within(() => {
      // Check that Last Error column (5th td, index 4) contains error text
      cy.get('td').eq(4).should('not.contain', '—').should('not.be.empty')
      // Check that Last Error At column (6th td, index 5) contains timestamp
      cy.get('td').eq(5).should('not.contain', '—')
      cy.get('td').eq(5).invoke('text').then((text) => {
        expect(text.trim()).to.match(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/)
      })
    })
    
    // Find the row with error500.xml feed and click Fetch button
    cy.contains('table tbody tr', 'error500.xml').within(() => {
      cy.get('form[action*="/fetch"] button[type="submit"]').click()
    })
    
    // Wait for redirect back to feeds page
    cy.url({ timeout: 10000 }).should('include', '/admin/feeds')
    
    // Verify error is displayed for 500 feed
    cy.contains('table tbody tr', 'error500.xml').within(() => {
      // Check that Last Error column (5th td, index 4) contains error text
      cy.get('td').eq(4).should('not.contain', '—').should('not.be.empty')
      // Check that Last Error At column (6th td, index 5) contains timestamp
      cy.get('td').eq(5).should('not.contain', '—')
      cy.get('td').eq(5).invoke('text').then((text) => {
        expect(text.trim()).to.match(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/)
      })
    })
    
    // Verify that error messages contain relevant information
    cy.contains('table tbody tr', 'error404.xml').within(() => {
      cy.get('td').eq(4).invoke('text').then((errorText) => {
        // Error should mention 404 or Not Found
        expect(errorText.toLowerCase()).to.satisfy((text) => {
          return text.includes('404') || text.includes('not found') || text.includes('error')
        })
      })
    })
    
    cy.contains('table tbody tr', 'error500.xml').within(() => {
      cy.get('td').eq(4).invoke('text').then((errorText) => {
        // Error should mention 500 or Internal Server Error
        expect(errorText.toLowerCase()).to.satisfy((text) => {
          return text.includes('500') || text.includes('internal server error') || text.includes('error')
        })
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

