describe('REST API', () => {
  before(() => {
    cy.clearCookies()
    cy.clearAllTables()
    cy.seedUsers()
  })

  describe('Authentication', () => {
    it('should return 401 for protected endpoints without auth', () => {
      cy.clearCookies()
      cy.apiRequest({
        method: 'GET',
        url: '/api/v1/users',
        failOnStatusCode: false,
      }).then((response) => {
        expect(response.status).to.eq(401)
        expect(response.body.error).to.eq('authentication required')
      })
    })

    it('should login via API and return user data', () => {
      cy.clearCookies()
      cy.apiLogin('admin', 'password').then((response) => {
        expect(response.body).to.have.property('id')
        expect(response.body.username).to.eq('admin')
      })
    })

    it('should reject invalid credentials', () => {
      cy.clearCookies()
      cy.apiRequest({
        method: 'POST',
        url: '/api/v1/auth/login',
        body: { username: 'admin', password: 'wrongpassword' },
        failOnStatusCode: false,
      }).then((response) => {
        expect(response.status).to.eq(401)
        expect(response.body.error).to.eq('invalid credentials')
      })
    })

    it('should logout via API', () => {
      cy.clearCookies()
      cy.apiLogin()
      cy.apiLogout()
      cy.apiRequest({
        method: 'GET',
        url: '/api/v1/users',
        failOnStatusCode: false,
      }).its('status').should('eq', 401)
    })
  })

  describe('Users API', () => {
    beforeEach(() => {
      cy.apiLoginWithSession()
    })

    it('should list users', () => {
      cy.apiRequest({ method: 'GET', url: '/api/v1/users' }).then((response) => {
        expect(response.status).to.eq(200)
        expect(response.body).to.have.property('items')
        expect(response.body.items).to.be.an('array')
        expect(response.body.items.length).to.be.at.least(1)
        expect(response.body.items[0]).to.have.property('username')
        expect(response.body.items[0]).not.to.have.property('password')
      })
    })

    it('should create, get, update and delete a user', () => {
      const username = `apiuser_${Date.now()}`
      const password = 'password123'

      cy.apiRequest({
        method: 'POST',
        url: '/api/v1/users',
        body: { username, password },
      }).then((createRes) => {
        expect(createRes.status).to.eq(201)
        expect(createRes.body.username).to.eq(username)
        const userId = createRes.body.id

        cy.apiRequest({ method: 'GET', url: `/api/v1/users/${userId}` }).then((getRes) => {
          expect(getRes.status).to.eq(200)
          expect(getRes.body.username).to.eq(username)
        })

        const updatedName = `updated_${username}`
        cy.apiRequest({
          method: 'PUT',
          url: `/api/v1/users/${userId}`,
          body: { username: updatedName, password: 'newpassword1' },
        }).then((updateRes) => {
          expect(updateRes.status).to.eq(200)
          expect(updateRes.body.username).to.eq(updatedName)
        })

        cy.apiRequest({
          method: 'DELETE',
          url: `/api/v1/users/${userId}`,
        }).then((deleteRes) => {
          expect(deleteRes.status).to.eq(204)
        })

        cy.apiRequest({
          method: 'GET',
          url: `/api/v1/users/${userId}`,
          failOnStatusCode: false,
        }).its('status').should('eq', 404)
      })
    })

    it('should reject invalid user data', () => {
      cy.apiRequest({
        method: 'POST',
        url: '/api/v1/users',
        body: { username: 'ab', password: 'short' },
        failOnStatusCode: false,
      }).then((response) => {
        expect(response.status).to.eq(400)
        expect(response.body.error).to.be.a('string').and.not.be.empty
      })
    })
  })

  describe('Feeds API', () => {
    beforeEach(() => {
      cy.apiLoginWithSession()
    })

    it('should create, list, get, update and delete a feed', () => {
      const feedUrl = `https://example.com/feed_${Date.now()}.xml`

      cy.apiRequest({
        method: 'POST',
        url: '/api/v1/feeds',
        body: { url: feedUrl },
      }).then((createRes) => {
        expect(createRes.status).to.eq(201)
        expect(createRes.body.url).to.eq(feedUrl)
        const feedId = createRes.body.id

        cy.apiRequest({ method: 'GET', url: '/api/v1/feeds' }).then((listRes) => {
          expect(listRes.status).to.eq(200)
          const urls = listRes.body.items.map((f) => f.url)
          expect(urls).to.include(feedUrl)
        })

        cy.apiRequest({ method: 'GET', url: `/api/v1/feeds/${feedId}` }).then((getRes) => {
          expect(getRes.status).to.eq(200)
          expect(getRes.body.url).to.eq(feedUrl)
        })

        const updatedUrl = `https://example.com/updated_${Date.now()}.xml`
        cy.apiRequest({
          method: 'PUT',
          url: `/api/v1/feeds/${feedId}`,
          body: { url: updatedUrl },
        }).then((updateRes) => {
          expect(updateRes.status).to.eq(200)
          expect(updateRes.body.url).to.eq(updatedUrl)
        })

        cy.apiRequest({
          method: 'DELETE',
          url: `/api/v1/feeds/${feedId}`,
        }).then((deleteRes) => {
          expect(deleteRes.status).to.eq(204)
        })

        cy.apiRequest({
          method: 'GET',
          url: `/api/v1/feeds/${feedId}`,
          failOnStatusCode: false,
        }).its('status').should('eq', 404)
      })
    })

    it('should reject duplicate feed URL', () => {
      const feedUrl = `https://example.com/dup_${Date.now()}.xml`

      cy.apiRequest({
        method: 'POST',
        url: '/api/v1/feeds',
        body: { url: feedUrl },
      }).its('status').should('eq', 201)

      cy.apiRequest({
        method: 'POST',
        url: '/api/v1/feeds',
        body: { url: feedUrl },
        failOnStatusCode: false,
      }).then((response) => {
        expect(response.status).to.eq(400)
        expect(response.body.error).to.eq('feed URL already exists')
      })
    })

    it('should fetch items from a test feed', () => {
      const feedUrl = `${Cypress.config('baseUrl')}/test_feeds/test1.xml`

      cy.apiRequest({
        method: 'POST',
        url: '/api/v1/feeds',
        body: { url: feedUrl },
      }).then((createRes) => {
        expect(createRes.status).to.eq(201)
        const feedId = createRes.body.id

        cy.apiRequest({
          method: 'POST',
          url: `/api/v1/feeds/${feedId}/fetch`,
        }).then((fetchRes) => {
          expect(fetchRes.status).to.eq(200)
          expect(fetchRes.body.items_created).to.be.at.least(1)
        })
      })
    })
  })

  describe('Items API', () => {
    let feedId

    before(() => {
      cy.apiLoginWithSession()
      const feedUrl = `${Cypress.config('baseUrl')}/test_feeds/test1.xml`

      cy.apiRequest({ method: 'GET', url: '/api/v1/feeds' }).then((listRes) => {
        const existing = listRes.body.items.find((f) => f.url === feedUrl)
        if (existing) {
          return cy.wrap(existing.id)
        }
        return cy.apiRequest({
          method: 'POST',
          url: '/api/v1/feeds',
          body: { url: feedUrl },
        }).then((res) => res.body.id)
      }).then((id) => {
        feedId = id
        cy.apiRequest({ method: 'POST', url: `/api/v1/feeds/${feedId}/fetch` })
      })
    })

    beforeEach(() => {
      cy.apiLoginWithSession()
    })

    it('should list items', () => {
      cy.apiRequest({ method: 'GET', url: '/api/v1/items' }).then((response) => {
        expect(response.status).to.eq(200)
        expect(response.body.items).to.be.an('array')
        expect(response.body.items.length).to.be.at.least(1)
        expect(response.body.items[0]).to.have.property('title')
        expect(response.body.items[0]).to.have.property('feed')
      })
    })

    it('should filter items by feed_id', () => {
      cy.apiRequest({
        method: 'GET',
        url: `/api/v1/items?feed_id=${feedId}`,
      }).then((response) => {
        expect(response.status).to.eq(200)
        expect(response.body.items.length).to.be.at.least(1)
        response.body.items.forEach((item) => {
          expect(item.feed_id).to.eq(feedId)
        })
      })
    })

    it('should get a single item', () => {
      cy.apiRequest({ method: 'GET', url: '/api/v1/items' }).then((listRes) => {
        const itemId = listRes.body.items[0].id
        cy.apiRequest({ method: 'GET', url: `/api/v1/items/${itemId}` }).then((getRes) => {
          expect(getRes.status).to.eq(200)
          expect(getRes.body.id).to.eq(itemId)
          expect(getRes.body.feed).to.have.property('url')
        })
      })
    })

    it('should return 404 for missing item', () => {
      cy.apiRequest({
        method: 'GET',
        url: '/api/v1/items/999999',
        failOnStatusCode: false,
      }).then((response) => {
        expect(response.status).to.eq(404)
        expect(response.body.error).to.eq('item not found')
      })
    })
  })

  describe('Swagger', () => {
    it('should serve swagger UI', () => {
      cy.request('/swagger/index.html').its('status').should('eq', 200)
    })

    it('should serve swagger JSON spec', () => {
      cy.request('/swagger/doc.json').then((response) => {
        expect(response.status).to.eq(200)
        expect(response.body.info.title).to.eq('Go RSS UI API')
        expect(response.body.paths).to.have.property('/auth/login')
        expect(response.body.paths).to.have.property('/users')
        expect(response.body.paths).to.have.property('/feeds')
        expect(response.body.paths).to.have.property('/items')
      })
    })
  })
})
