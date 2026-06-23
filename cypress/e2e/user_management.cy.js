describe('User Management', () => {
  before(() => {
    cy.clearAllTables()
    cy.seedUsers()    
  })

  beforeEach(() => {
    cy.loginWithSession()
  })

  describe('Create User', () => {
    it('should display create user form', () => {
      cy.visit('/admin/users')
      cy.get('a[href="/admin/users/new"]').first().should('be.visible').click()
      cy.url().should('include', '/admin/users/new')
      cy.get('h1').contains('Create New User').should('be.visible')
      cy.get('input[name="username"]').should('be.visible')
      cy.get('input[name="password"]').should('be.visible')
      cy.get('form[action="/admin/users"] button[type="submit"]').should('be.visible').should('contain', 'Create User')
      cy.get('form[action="/admin/users"] a[href="/admin/users"]').should('be.visible').should('contain', 'Cancel')
    })

    it('should create a new user successfully', () => {
      cy.visit('/admin/users/new')
      const username = `testuser_${Date.now()}`
      cy.get('input[name="username"]').type(username)
      cy.get('input[name="password"]').type('testpassword123')
      cy.get('form[action="/admin/users"] button[type="submit"]').click()
      
      cy.url().should('include', '/admin/users')
      cy.get('tbody tr').should('contain', username)
    })

    it('should show error when creating user with empty username', () => {
      cy.visit('/admin/users/new')
      cy.get('input[name="password"]').type('testpassword123')
      cy.get('form[action="/admin/users"] button[type="submit"]').click()
      
      // HTML5 validation should prevent submission, but if it goes through, check for error
      cy.get('input[name="username"]:invalid').should('exist')
    })

    it('should show error when creating user with empty password', () => {
      cy.visit('/admin/users/new')
      cy.get('input[name="username"]').type('testuser')
      cy.get('form[action="/admin/users"] button[type="submit"]').click()
      
      // HTML5 validation should prevent submission
      cy.get('input[name="password"]:invalid').should('exist')
    })

    it('should show error when creating user with duplicate username', () => {
      const username = `duplicate_${Date.now()}`
      
      // Create first user
      cy.visit('/admin/users/new')
      cy.get('input[name="username"]').type(username)
      cy.get('input[name="password"]').type('password1')
      cy.get('form[action="/admin/users"] button[type="submit"]').click()
      cy.url().should('include', '/admin/users')
      
      // Verify user was created
      cy.get('tbody tr').filter(`:contains("${username}")`).should('have.length', 1)
      
      // Try to create duplicate
      cy.visit('/admin/users/new')
      cy.get('input[name="username"]').type(username)
      cy.get('input[name="password"]').type('password2')
      cy.get('form[action="/admin/users"] button[type="submit"]').click()
      
      cy.get('h1').contains('Create New User').should('be.visible')
      cy.get('.alert-danger').should('be.visible').should('contain', 'Username already exists')
      cy.visit('/admin/users')
      cy.get('tbody tr').filter(`:contains("${username}")`).should('have.length', 1)
    })

    it('should cancel create user and return to users list', () => {
      cy.visit('/admin/users/new')
      cy.get('form[action="/admin/users"] a[href="/admin/users"]').first().click()
      cy.url().should('eq', Cypress.config('baseUrl') + '/admin/users')
      cy.get('h1').contains('User Management').should('be.visible')
    })
  })

  describe('Edit User', () => {
    let testUserId
    let testUsername

    beforeEach(() => {
      // Create a test user for editing
      testUsername = `edittest_${Date.now()}`
      cy.visit('/admin/users/new')
      cy.get('input[name="username"]').type(testUsername)
      cy.get('input[name="password"]').type('originalpassword')
      cy.get('form[action="/admin/users"] button[type="submit"]').click()
      
      // Get the user ID from the table
      cy.visit('/admin/users')
      cy.get('tbody tr').contains(testUsername).parent('tr').find('td').first().then(($td) => {
        testUserId = $td.text().trim()
      })
    })

    it('should display edit user form', () => {
      cy.visit('/admin/users')
      cy.get('tbody tr').contains(testUsername).parent('tr').find('a[href*="/edit"]').click()
      cy.url().should('include', `/admin/users/${testUserId}/edit`)
      cy.get('h1').contains('Edit User').should('be.visible')
      cy.get('input[name="username"]').should('have.value', testUsername)
      cy.get('input[name="password"]').should('be.visible')
      cy.get('form[action*="/edit"] button[type="submit"]').should('be.visible').should('contain', 'Update User')
      cy.get('form[action*="/edit"] a[href="/admin/users"]').should('be.visible').should('contain', 'Cancel')
    })

    it('should update username successfully', () => {
      const newUsername = `updated_${Date.now()}`
      cy.visit(`/admin/users/${testUserId}/edit`)
      cy.get('input[name="username"]').clear().type(newUsername)
      cy.get('form[action*="/edit"] button[type="submit"]').click()
      
      cy.url().should('include', '/admin/users')
      cy.get('tbody tr').should('contain', newUsername)
      cy.get('tbody tr').should('not.contain', testUsername)
    })

    it('should update password successfully', () => {
      const newPassword = 'newpassword123'
      cy.visit(`/admin/users/${testUserId}/edit`)
      cy.get('input[name="password"]').type(newPassword)
      cy.get('form[action*="/edit"] button[type="submit"]').click()
      
      cy.url().should('include', '/admin/users')
      cy.get('tbody tr').should('contain', testUsername)
      
      // Verify password was changed by logging in with new password
      cy.logout()
      cy.login(testUsername, newPassword)
      cy.shouldBeLoggedIn()
    })

    it('should update both username and password', () => {
      const newUsername = `bothupdated_${Date.now()}`
      const newPassword = 'newpass123'
      cy.visit(`/admin/users/${testUserId}/edit`)
      cy.get('input[name="username"]').clear().type(newUsername)
      cy.get('input[name="password"]').type(newPassword)
      cy.get('form[action*="/edit"] button[type="submit"]').click()
      
      cy.url().should('include', '/admin/users')
      cy.get('tbody tr').should('contain', newUsername)
      
      // Verify password was changed
      cy.logout()
      cy.login(newUsername, newPassword)
      cy.shouldBeLoggedIn()
    })

    it('should keep password unchanged when password field is empty', () => {
      const originalPassword = 'originalpassword'
      const newUsername = `keptpass_${Date.now()}`
      cy.visit(`/admin/users/${testUserId}/edit`)
      cy.get('input[name="username"]').clear().type(newUsername)
      // Don't fill password field - leave it empty
      cy.get('form[action*="/edit"] button[type="submit"]').click()
      
      cy.url().should('include', '/admin/users')
      
      // Verify original password still works with new username
      cy.logout()
      cy.login(newUsername, originalPassword)
      cy.shouldBeLoggedIn()
    })

    it('should cancel edit and return to users list', () => {
      cy.visit(`/admin/users/${testUserId}/edit`)
      cy.get('form[action*="/edit"] a[href="/admin/users"]').first().click()
      cy.url().should('eq', Cypress.config('baseUrl') + '/admin/users')
      cy.get('h1').contains('User Management').should('be.visible')
    })

    it('should show error when editing non-existent user', () => {
      cy.visit('/admin/users/99999/edit', { failOnStatusCode: false })
      cy.get('.alert-danger').should('be.visible').should('contain', 'User not found')
      cy.url().should('include', '/admin/users')
    })
  })

  describe('Delete User', () => {
    let testUserId
    let testUsername

    beforeEach(() => {
      // Create a test user for deletion
      testUsername = `deletetest_${Date.now()}`
      cy.visit('/admin/users/new')
      cy.get('input[name="username"]').type(testUsername)
      cy.get('input[name="password"]').type('password123')
      cy.get('form[action="/admin/users"] button[type="submit"]').click()
      
      // Get the user ID from the table
      cy.visit('/admin/users')
      cy.get('tbody tr').contains(testUsername).parent('tr').find('td').first().then(($td) => {
        testUserId = $td.text().trim()
      })
    })

    it('should delete user successfully', () => {
      cy.visit('/admin/users')
      cy.get('tbody tr').contains(testUsername).should('exist')
      
      cy.stubConfirm(true)
      cy.get('tbody tr').contains(testUsername).parent('tr').find('form[action*="/delete"] button').click()
      
      cy.url().should('include', '/admin/users')
      cy.get('tbody tr').should('not.contain', testUsername)
    })

    it('should cancel delete when confirmation is rejected', () => {
      cy.visit('/admin/users')
      cy.get('tbody tr').contains(testUsername).should('exist')
      
      cy.stubConfirm(false)
      cy.get('tbody tr').contains(testUsername).parent('tr').find('form[action*="/delete"] button').click()
      
      // User should still exist
      cy.get('tbody tr').should('contain', testUsername)
    })

    it('should show error when deleting non-existent user', () => {
      cy.request({
        method: 'POST',
        url: '/admin/users/99999/delete',
        followRedirect: false,
      }).then((response) => {
        expect(response.status).to.eq(302)
        expect(response.headers.location).to.include('/admin/users')
      })
      cy.visit('/admin/users')
      cy.get('.alert-danger').should('be.visible').should('contain', 'User not found')
    })
  })

  describe('User List Navigation', () => {
    it('should have link to create new user', () => {
      cy.visit('/admin/users')
      cy.get('a[href="/admin/users/new"]').should('be.visible').should('contain', 'Create New User')
    })

    it('should have edit and delete links for each user', () => {
      cy.visit('/admin/users')
      cy.get('tbody tr').first().within(() => {
        cy.get('a[href*="/edit"]').should('be.visible').should('contain', 'Edit')
        cy.get('form[action*="/delete"]').should('be.visible')
        cy.get('form[action*="/delete"] button').should('be.visible').should('contain', 'Delete')
      })
    })

    it('should redirect from /admin to /admin/users', () => {
      cy.visit('/admin')
      cy.url().should('include', '/admin/users')
      cy.get('h1').contains('User Management').should('be.visible')
    })
  })
})

