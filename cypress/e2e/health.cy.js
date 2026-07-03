describe('Health probes', () => {
  beforeEach(() => {
    cy.clearCookies()
  })

  it('HEAD / returns 200 without session cookie', () => {
    cy.request({
      method: 'HEAD',
      url: '/',
    }).then((response) => {
      expect(response.status).to.eq(200)
      expect(response.body).to.eq('')
      expect(response.headers).not.to.have.property('set-cookie')
    })
  })

  it('GET /health returns 200 with ok body and no session cookie', () => {
    cy.request({
      method: 'GET',
      url: '/health',
    }).then((response) => {
      expect(response.status).to.eq(200)
      expect(response.body).to.eq('ok')
      expect(response.headers).not.to.have.property('set-cookie')
    })
  })

  it('HEAD /health returns 200 without body or session cookie', () => {
    cy.request({
      method: 'HEAD',
      url: '/health',
    }).then((response) => {
      expect(response.status).to.eq(200)
      expect(response.body).to.eq('')
      expect(response.headers).not.to.have.property('set-cookie')
    })
  })
})
