describe('Items Management', () => {
  beforeEach(() => {
    cy.clearUsersLoginRememberSession()
  })

  describe('Items List', () => {
    it('should display items list page', () => {
      cy.visit('/admin/items')
      cy.get('h1').contains('Items').should('be.visible')
      cy.get('table').should('be.visible')
      cy.get('table thead tr').should('contain', 'ID')
      cy.get('table thead tr').should('contain', 'Title')
      cy.get('table thead tr').should('contain', 'Feed')
      cy.get('table thead tr').should('contain', 'Author')
      cy.get('table thead tr').should('contain', 'Published At')
      cy.get('table thead tr').should('contain', 'Created At')
      cy.get('table thead tr').should('contain', 'Actions')
    })

    it('should have navigation links', () => {
      cy.visit('/admin/items')
      cy.get('a[href="/admin/users"]').should('be.visible')
      cy.get('a[href="/admin/feeds"]').should('be.visible')
    })

    it('should display empty state when no items exist', () => {
      cy.visit('/admin/items')
      // Table should exist even if empty
      cy.get('table').should('be.visible')
      cy.get('tbody').should('exist')
    })
  })

  describe('View Item', () => {
    it('should show error when viewing non-existent item', () => {
      cy.visit('/admin/items/99999', { failOnStatusCode: false })
      // Should stay on the same URL (no redirect)
      cy.url().should('include', '/admin/items/99999')
      // Should return 404 status
      cy.request({
        url: '/admin/items/99999',
        failOnStatusCode: false
      }).then((response) => {
        expect(response.status).to.eq(404)
      })
      // Should show error message
      cy.get('.error').should('be.visible').should('contain', 'Item not found')
    })

    it('should display item detail page when items exist', () => {
      cy.visit('/admin/items')
      // Check if any items exist by looking for View links
      cy.get('body').then(($body) => {
        if ($body.find('a[href*="/items/"]').length > 0) {
          // Items exist - test viewing one
          cy.get('tbody tr').first().within(() => {
            cy.get('a[href*="/items/"]').invoke('attr', 'href').then((href) => {
              const itemId = href.split('/').pop()
              cy.visit(`/admin/items/${itemId}`)
              cy.get('h2').should('be.visible')
              cy.get('a[href="/admin/items"]').should('be.visible').should('contain', 'Back to Items')
            })
          })
        } else {
          // No items - just verify items page loads correctly
          cy.get('h1').contains('Items').should('be.visible')
          cy.get('table').should('be.visible')
        }
      })
    })
  })

  describe('Items with Feeds', () => {
    it('should display feed information for items', () => {
      cy.visit('/admin/items')
      // Verify table structure includes Feed column
      cy.get('table thead tr').should('contain', 'Feed')
      // If items exist, verify feed column has content
      cy.get('body').then(($body) => {
        if ($body.find('tbody tr').length > 0) {
          cy.get('tbody tr').first().find('td').eq(2).should('exist') // Feed column
        }
      })
    })
  })

  describe('Delete All Items', () => {
    beforeEach(() => {
      // Create a feed and fetch items to have some items to delete
      const feedUrl = `http://localhost:8082/test_feeds/test1.xml`
      cy.visit('/admin/feeds/new')
      cy.get('input[name="url"]').type(feedUrl)
      cy.get('form[action="/admin/feeds"] button[type="submit"]').click()
      cy.url().should('include', '/admin/feeds')
      
      // Fetch items from the feed
      cy.visit('/admin/items')
      cy.get('form[action="/admin/items/fetch"] button').click()
      cy.url().should('include', '/admin/items')
      
      // Wait for items to be fetched
      cy.wait(2000)
    })

    it('should display Delete All Items button', () => {
      cy.visit('/admin/items')
      cy.get('form[action="/admin/items/delete-all"]').should('be.visible')
      cy.get('form[action="/admin/items/delete-all"] button').should('be.visible').should('contain', 'Delete All Items')
    })

    it('should delete all items when confirmed', () => {
      cy.visit('/admin/items')
      
      // Verify items exist
      cy.get('tbody tr').then(($rows) => {
        const initialCount = $rows.length
        expect(initialCount).to.be.at.least(1)
        
        // Intercept the confirm dialog and accept it
        cy.window().then((win) => {
          cy.stub(win, 'confirm').returns(true)
        })
        
        cy.get('form[action="/admin/items/delete-all"] button').click()
        
        cy.url().should('include', '/admin/items')
        // All items should be deleted
        cy.get('tbody tr').should('have.length', 0)
      })
    })

    it('should not delete items when confirmation is rejected', () => {
      cy.visit('/admin/items')
      
      // Count items before
      cy.get('tbody tr').then(($rows) => {
        const initialCount = $rows.length
        expect(initialCount).to.be.at.least(1)
        
        // Intercept the confirm dialog and reject it
        cy.window().then((win) => {
          cy.stub(win, 'confirm').returns(false)
        })
        
        cy.get('form[action="/admin/items/delete-all"] button').click()
        
        // Items should still exist
        cy.get('tbody tr').should('have.length', initialCount)
      })
    })
  })
})

