/**
 * Component tests for Agent Dashboard
 */

import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Skeleton, SkeletonCard, SkeletonList, SkeletonMessage } from '../components/ui/skeleton';
import { EmptyState } from '../components/empty-state';

describe('Button component', () => {
  it('renders with default variant', () => {
    render(<Button>Click me</Button>);
    const button = screen.getByRole('button', { name: /click me/i });
    expect(button).toBeInTheDocument();
  });

  it('renders with different variants', () => {
    const { rerender } = render(<Button variant="destructive">Delete</Button>);
    expect(screen.getByRole('button')).toBeInTheDocument();

    rerender(<Button variant="outline">Outline</Button>);
    expect(screen.getByRole('button')).toBeInTheDocument();

    rerender(<Button variant="secondary">Secondary</Button>);
    expect(screen.getByRole('button')).toBeInTheDocument();

    rerender(<Button variant="ghost">Ghost</Button>);
    expect(screen.getByRole('button')).toBeInTheDocument();

    rerender(<Button variant="link">Link</Button>);
    expect(screen.getByRole('button')).toBeInTheDocument();
  });

  it('renders with different sizes', () => {
    const { rerender } = render(<Button size="sm">Small</Button>);
    expect(screen.getByRole('button')).toBeInTheDocument();

    rerender(<Button size="lg">Large</Button>);
    expect(screen.getByRole('button')).toBeInTheDocument();

    rerender(<Button size="icon">Icon</Button>);
    expect(screen.getByRole('button')).toBeInTheDocument();
  });

  it('handles click events', () => {
    const handleClick = jest.fn();
    render(<Button onClick={handleClick}>Click me</Button>);

    fireEvent.click(screen.getByRole('button'));
    expect(handleClick).toHaveBeenCalledTimes(1);
  });

  it('can be disabled', () => {
    const handleClick = jest.fn();
    render(
      <Button disabled onClick={handleClick}>
        Disabled
      </Button>
    );

    const button = screen.getByRole('button');
    expect(button).toBeDisabled();

    fireEvent.click(button);
    expect(handleClick).not.toHaveBeenCalled();
  });

  it('renders as child element when asChild is true', () => {
    render(
      <Button asChild>
        <a href="/test">Link Button</a>
      </Button>
    );
    expect(screen.getByRole('link')).toBeInTheDocument();
  });
});

describe('Input component', () => {
  it('renders correctly', () => {
    render(<Input placeholder="Enter text" />);
    expect(screen.getByPlaceholderText('Enter text')).toBeInTheDocument();
  });

  it('handles value changes', () => {
    const handleChange = jest.fn();
    render(<Input onChange={handleChange} />);

    const input = screen.getByRole('textbox');
    fireEvent.change(input, { target: { value: 'test' } });

    expect(handleChange).toHaveBeenCalled();
  });

  it('can be disabled', () => {
    render(<Input disabled />);
    expect(screen.getByRole('textbox')).toBeDisabled();
  });

  it('supports different types', () => {
    const { rerender } = render(<Input type="email" />);
    expect(screen.getByRole('textbox')).toHaveAttribute('type', 'email');

    rerender(<Input type="password" />);
    // password inputs don't have textbox role
  });

  it('forwards ref correctly', () => {
    const ref = React.createRef<HTMLInputElement>();
    render(<Input ref={ref} />);
    expect(ref.current).toBeInstanceOf(HTMLInputElement);
  });
});

describe('Skeleton components', () => {
  describe('Skeleton', () => {
    it('renders with custom className', () => {
      render(<Skeleton className="custom-class" data-testid="skeleton" />);
      expect(screen.getByTestId('skeleton')).toHaveClass('custom-class');
    });
  });

  describe('SkeletonCard', () => {
    it('renders card skeleton', () => {
      render(<SkeletonCard />);
      // SkeletonCard renders multiple skeleton elements
      expect(document.querySelectorAll('.animate-pulse').length).toBeGreaterThan(0);
    });
  });

  describe('SkeletonList', () => {
    it('renders specified number of items', () => {
      render(<SkeletonList count={5} />);
      // SkeletonList renders SkeletonCard items inside space-y-4 container
      const container = document.querySelector('.space-y-4');
      expect(container).toBeInTheDocument();
      // Each SkeletonCard is a direct child of the container
      const cards = container?.querySelectorAll(':scope > div');
      expect(cards).toHaveLength(5);
    });

    it('defaults to 3 items', () => {
      render(<SkeletonList />);
      const container = document.querySelector('.space-y-4');
      const cards = container?.querySelectorAll(':scope > div');
      expect(cards).toHaveLength(3);
    });
  });

  describe('SkeletonMessage', () => {
    it('renders message skeleton', () => {
      render(<SkeletonMessage />);
      expect(document.querySelector('.animate-pulse')).toBeInTheDocument();
    });

    it('renders with correct structure', () => {
      render(<SkeletonMessage />);
      // SkeletonMessage has a rounded avatar and content skeletons
      expect(document.querySelector('.rounded-full')).toBeInTheDocument();
    });
  });
});

describe('EmptyState component', () => {
  it('renders with title and description', () => {
    render(
      <EmptyState
        title="No items found"
        description="Try adding some items to see them here."
      />
    );

    expect(screen.getByText('No items found')).toBeInTheDocument();
    expect(screen.getByText('Try adding some items to see them here.')).toBeInTheDocument();
  });

  it('renders with action button', () => {
    const handleAction = jest.fn();
    render(
      <EmptyState
        title="No items"
        description="Add some items"
        action={{
          label: 'Add Item',
          onClick: handleAction,
        }}
      />
    );

    const button = screen.getByRole('button', { name: /add item/i });
    expect(button).toBeInTheDocument();

    fireEvent.click(button);
    expect(handleAction).toHaveBeenCalledTimes(1);
  });

  it('renders with custom icon', () => {
    const CustomIcon = () => <div data-testid="custom-icon">Icon</div>;
    render(
      <EmptyState
        title="No items"
        description="Add some items"
        icon={CustomIcon as unknown as import('lucide-react').LucideIcon}
      />
    );

    expect(screen.getByTestId('custom-icon')).toBeInTheDocument();
  });
});

describe('Component accessibility', () => {
  it('Button has correct role', () => {
    render(<Button>Accessible Button</Button>);
    expect(screen.getByRole('button')).toBeInTheDocument();
  });

  it('Input has correct role', () => {
    render(<Input />);
    expect(screen.getByRole('textbox')).toBeInTheDocument();
  });

  it('Input supports aria-label', () => {
    render(<Input aria-label="Email address" />);
    expect(screen.getByLabelText('Email address')).toBeInTheDocument();
  });

  it('Button supports aria-disabled', () => {
    render(<Button disabled aria-disabled="true">Disabled</Button>);
    expect(screen.getByRole('button')).toHaveAttribute('disabled');
  });
});

describe('Component styling', () => {
  it('Button applies className prop', () => {
    render(<Button className="custom-button">Styled</Button>);
    expect(screen.getByRole('button')).toHaveClass('custom-button');
  });

  it('Input applies className prop', () => {
    render(<Input className="custom-input" />);
    expect(screen.getByRole('textbox')).toHaveClass('custom-input');
  });
});
