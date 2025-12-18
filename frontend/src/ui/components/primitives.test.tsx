import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { Button, ButtonLink, FormField, Input } from './index'

describe('ui/components primitives', () => {
  it('Button defaults to type=button and exposes variant/size for debugging', () => {
    render(
      <Button variant="primary" size="sm">
        Save
      </Button>,
    )

    const button = screen.getByRole('button', { name: /save/i })
    expect(button).toHaveAttribute('type', 'button')
    expect(button).toHaveAttribute('data-variant', 'primary')
    expect(button).toHaveAttribute('data-size', 'sm')
  })

  it('ButtonLink renders a link with button styling hooks', () => {
    render(
      <MemoryRouter>
        <ButtonLink to="/recipes" variant="ghost" size="md">
          Recipes
        </ButtonLink>
      </MemoryRouter>,
    )

    const link = screen.getByRole('link', { name: /recipes/i })
    expect(link).toHaveAttribute('href', '/recipes')
    expect(link).toHaveAttribute('data-variant', 'ghost')
    expect(link).toHaveAttribute('data-size', 'md')
  })

  it('FormField wires label, describedBy, and aria-invalid', () => {
    render(
      <FormField
        label="Username"
        description="Used to sign in."
        error="Username is required."
        required
      >
        {({ id, describedBy, invalid }) => (
          <Input id={id} aria-describedby={describedBy} invalid={invalid} />
        )}
      </FormField>,
    )

    const input = screen.getByLabelText(/username/i)
    expect(input).toHaveAttribute('aria-invalid', 'true')

    const describedBy = input.getAttribute('aria-describedby') ?? ''
    const describedIDs = describedBy.split(' ').filter(Boolean)

    const description = screen.getByText(/used to sign in/i)
    const error = screen.getByRole('alert')

    expect(description).toHaveAttribute(
      'id',
      expect.stringMatching(/-description$/),
    )
    expect(error).toHaveAttribute('id', expect.stringMatching(/-error$/))

    const descriptionID = description.getAttribute('id')
    const errorID = error.getAttribute('id')
    expect(descriptionID).not.toBeNull()
    expect(errorID).not.toBeNull()

    expect(describedIDs).toEqual(
      expect.arrayContaining([descriptionID!, errorID!]),
    )

    expect(error).toHaveTextContent(/username is required/i)
  })
})
