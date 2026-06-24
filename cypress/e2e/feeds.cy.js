describe('Feed Management', () => {
  before(() => {
    cy.clearAllTables()
    cy.seedUsers()
  })

  beforeEach(() => {
    cy.loginWithSession()
  })

  describe('Feed List', () => {
    it('should display feeds list page', () => {
      cy.visit('/admin/feeds')
      cy.get('h1').contains('Feed Management').should('be.visible')
      cy.get('a[href="/admin/feeds/new"]').should('be.visible').should('contain', 'Create New Feed')
      cy.get('table').should('be.visible')
      cy.get('table thead tr').should('contain', 'ID')
      cy.get('table thead tr').should('contain', 'URL')
      cy.get('table thead tr').should('contain', 'Title')
      cy.get('table thead tr').should('contain', 'Created At')
      cy.get('table thead tr').should('contain', 'Actions')
    })

    it('should have navigation links', () => {
      cy.visit('/admin/feeds')
      cy.get('a[href="/admin/users"]').should('be.visible')
      cy.get('a[href="/admin/items"]').should('be.visible')
    })
  })

  describe('Create Feed', () => {
    it('should display create feed form', () => {
      cy.visit('/admin/feeds')
      cy.get('a[href="/admin/feeds/new"]').first().click()
      cy.url().should('include', '/admin/feeds/new')
      cy.get('h1').contains('Create New Feed').should('be.visible')
      cy.get('input[name="url"]').should('be.visible')
      cy.get('form[action="/admin/feeds"] button[type="submit"]').should('be.visible').should('contain', 'Create Feed')
      cy.get('form[action="/admin/feeds"] a[href="/admin/feeds"]').should('be.visible').should('contain', 'Cancel')
    })

    it('should create a new feed successfully', () => {
      cy.visit('/admin/feeds/new')
      const feedUrl = `https://example.com/rss_${Date.now()}.xml`
      cy.get('input[name="url"]').type(feedUrl)
      cy.get('form[action="/admin/feeds"] button[type="submit"]').click()
      
      cy.url().should('include', '/admin/feeds')
      cy.get('tbody tr').should('contain', feedUrl)
    })

    it('should show error when creating feed with empty URL', () => {
      cy.visit('/admin/feeds/new')
      cy.get('form[action="/admin/feeds"] button[type="submit"]').click()
      
      // HTML5 validation should prevent submission
      cy.get('input[name="url"]:invalid').should('exist')
    })

    it('should show error when creating feed with duplicate URL', () => {
      const feedUrl = `https://example.com/duplicate_${Date.now()}.xml`
      
      // Create first feed
      cy.visit('/admin/feeds/new')
      cy.get('input[name="url"]').type(feedUrl)
      cy.get('form[action="/admin/feeds"] button[type="submit"]').click()
      cy.url().should('include', '/admin/feeds')
      
      // Verify feed was created
      cy.get('tbody tr').filter(`:contains("${feedUrl}")`).should('have.length', 1)
      
      // Try to create duplicate
      cy.visit('/admin/feeds/new')
      cy.get('input[name="url"]').type(feedUrl)
      cy.get('form[action="/admin/feeds"] button[type="submit"]').click()
      
      cy.get('h1').contains('Create New Feed').should('be.visible')
      cy.get('.alert-danger').should('be.visible').should('contain', 'Feed URL already exists')
      cy.visit('/admin/feeds')
      cy.get('tbody tr').filter(`:contains("${feedUrl}")`).should('have.length', 1)
    })

    it('should cancel create feed and return to feeds list', () => {
      cy.visit('/admin/feeds/new')
      cy.get('form[action="/admin/feeds"] a[href="/admin/feeds"]').first().click()
      cy.url().should('eq', Cypress.config('baseUrl') + '/admin/feeds')
      cy.get('h1').contains('Feed Management').should('be.visible')
    })
  })

  describe('Edit Feed', () => {
    let testFeedId
    let testFeedUrl

    beforeEach(() => {
      testFeedUrl = `https://example.com/edittest_${Date.now()}.xml`
      cy.visit('/admin/feeds/new')
      cy.get('input[name="url"]').type(testFeedUrl)
      cy.get('form[action="/admin/feeds"] button[type="submit"]').click()
      cy.visit('/admin/feeds')
      cy.get('tbody tr').contains(testFeedUrl).parent('tr').find('td').first().then(($td) => {
        testFeedId = $td.text().trim()
      })
    })

    it('should display edit feed form', () => {
      cy.visit('/admin/feeds')
      cy.get('tbody tr').contains(testFeedUrl).parent('tr').find('a[href*="/edit"]').click()
      cy.url().should('include', `/admin/feeds/${testFeedId}/edit`)
      cy.get('h1').contains('Edit Feed').should('be.visible')
      cy.get('input[name="url"]').should('have.value', testFeedUrl)
      cy.get('form[action*="/edit"] button[type="submit"]').should('be.visible').should('contain', 'Update Feed')
      cy.get('form[action*="/edit"] a[href="/admin/feeds"]').should('be.visible').should('contain', 'Cancel')
    })

    it('should update feed URL successfully', () => {
      const newFeedUrl = `https://example.com/updated_${Date.now()}.xml`
      cy.visit(`/admin/feeds/${testFeedId}/edit`)
      cy.get('input[name="url"]').clear().type(newFeedUrl)
      cy.get('form[action*="/edit"] button[type="submit"]').click()

      cy.url().should('include', '/admin/feeds')
      cy.get('tbody tr').should('contain', newFeedUrl)
      cy.get('tbody tr').should('not.contain', testFeedUrl)
    })

    it('should show error when updating feed with duplicate URL', () => {
      const otherFeedUrl = `https://example.com/other_${Date.now()}.xml`
      cy.visit('/admin/feeds/new')
      cy.get('input[name="url"]').type(otherFeedUrl)
      cy.get('form[action="/admin/feeds"] button[type="submit"]').click()

      cy.visit(`/admin/feeds/${testFeedId}/edit`)
      cy.get('input[name="url"]').clear().type(otherFeedUrl)
      cy.get('form[action*="/edit"] button[type="submit"]').click()

      cy.get('h1').contains('Edit Feed').should('be.visible')
      cy.get('.alert-danger').should('be.visible').should('contain', 'Feed URL already exists')
      cy.visit('/admin/feeds')
      cy.get('tbody tr').should('contain', testFeedUrl)
      cy.get('tbody tr').should('contain', otherFeedUrl)
    })

    it('should cancel edit and return to feeds list', () => {
      cy.visit(`/admin/feeds/${testFeedId}/edit`)
      cy.get('form[action*="/edit"] a[href="/admin/feeds"]').first().click()
      cy.url().should('eq', Cypress.config('baseUrl') + '/admin/feeds')
      cy.get('h1').contains('Feed Management').should('be.visible')
    })

    it('should show error when editing non-existent feed', () => {
      cy.visit('/admin/feeds/99999/edit', { failOnStatusCode: false })
      cy.url().should('include', '/admin/feeds')
      cy.get('.alert-danger').should('be.visible').should('contain', 'Feed not found')
    })
  })

  describe('Delete Feed', () => {
    let testFeedUrl

    beforeEach(() => {
      testFeedUrl = `https://example.com/deletetest_${Date.now()}.xml`
      cy.visit('/admin/feeds/new')
      cy.get('input[name="url"]').type(testFeedUrl)
      cy.get('form[action="/admin/feeds"] button[type="submit"]').click()
      cy.visit('/admin/feeds')
      cy.get('tbody tr').contains(testFeedUrl).parent('tr').find('td').first().invoke('text').as('testFeedId')
    })

    it('should display feed detail page by id', () => {
      cy.get('@testFeedId').then((id) => {
        const feedId = id.trim()
        cy.visit(`/admin/feeds/${feedId}`)
        cy.url().should('include', `/admin/feeds/${feedId}`)
        cy.get('h2').contains('Feed Information').should('be.visible')
        cy.contains('dd', testFeedUrl).should('be.visible')
      })
    })

    it('should delete feed successfully', () => {
      cy.visit('/admin/feeds')
      cy.get('tbody tr').contains(testFeedUrl).should('exist')
      
      cy.stubConfirm(true)
      cy.get('tbody tr').contains(testFeedUrl).parent('tr').find('form[action*="/delete"] button').click()
      
      cy.url().should('include', '/admin/feeds')
      // Check that the feed is no longer in the table
      // If table is empty, tbody might not have any tr elements
      cy.get('tbody').then(($tbody) => {
        if ($tbody.find('tr').length > 0) {
          // Table has rows, verify testFeedUrl is not in any of them
          cy.get('tbody tr').should('not.contain', testFeedUrl)
        } else {
          // Table is empty, which is fine - feed was deleted
          cy.get('tbody').should('exist')
        }
      })
    })

    it('should cancel delete when confirmation is rejected', () => {
      cy.visit('/admin/feeds')
      cy.get('tbody tr').contains(testFeedUrl).should('exist')
      
      cy.stubConfirm(false)
      cy.get('tbody tr').contains(testFeedUrl).parent('tr').find('form[action*="/delete"] button').click()
      
      // Feed should still exist
      cy.get('tbody tr').should('contain', testFeedUrl)
    })
  })

  describe('Delete All Feeds', () => {
    beforeEach(() => {
      // Create some test feeds
      const feedUrls = [
        `https://example123.com/deleteall1_${Date.now()}.xml`,
        `https://example123.com/deleteall2_${Date.now()}.xml`,
        `https://example123.com/deleteall3_${Date.now()}.xml`
      ]
      
      feedUrls.forEach((feedUrl) => {
        cy.visit('/admin/feeds/new')
        cy.get('input[name="url"]').type(feedUrl)
        cy.get('form[action="/admin/feeds"] button[type="submit"]').click()
        cy.url().should('include', '/admin/feeds')
      })
    })

    it('should display Delete All Feeds button', () => {
      cy.visit('/admin/feeds')
      cy.get('form[action="/admin/feeds/delete-all"]').should('be.visible')
      cy.get('form[action="/admin/feeds/delete-all"] button').should('be.visible').should('contain', 'Delete All Feeds')
    })

    it('should delete all feeds when confirmed', () => {
      cy.visit('/admin/feeds')
      
      // Get initial count of feeds
      cy.get('tbody tr').then(($rows) => {
        const initialCount = $rows.length
        expect(initialCount).to.be.at.least(3)
        
        cy.stubConfirm(true)
        cy.get('form[action="/admin/feeds/delete-all"] button').click()
        
        cy.url().should('include', '/admin/feeds')
        // All feeds should be deleted
        cy.get('tbody tr').should('have.length', 0)
      })
    })

    it('should not delete feeds when confirmation is rejected', () => {
      cy.visit('/admin/feeds')
      
      // Count feeds before
      cy.get('tbody tr').then(($rows) => {
        const initialCount = $rows.length
        expect(initialCount).to.be.at.least(3)
        
        cy.stubConfirm(false)
        cy.get('form[action="/admin/feeds/delete-all"] button').click()
        
        // Feeds should still exist
        cy.get('tbody tr').should('have.length', initialCount)
      })
    })
  })
})

