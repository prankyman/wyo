import { beforeEach, describe, expect, it } from 'vitest';
import { toast } from './toast.svelte';

describe('Toast Store', () => {
    beforeEach(() => {
        // Clear toasts before each test
        toast.toasts = [];
    });

    it('adds a success toast', () => {
        toast.success('Success message');
        expect(toast.toasts).toHaveLength(1);
        expect(toast.toasts[0].title).toBe('Success message');
        expect(toast.toasts[0].variant).toBe('success');
    });

    it('adds an error toast', () => {
        toast.error('Error message');
        expect(toast.toasts).toHaveLength(1);
        expect(toast.toasts[0].title).toBe('Error message');
        expect(toast.toasts[0].variant).toBe('destructive');
    });

    it('adds a warning toast', () => {
        toast.warning('Warning message');
        expect(toast.toasts).toHaveLength(1);
        expect(toast.toasts[0].title).toBe('Warning message');
        expect(toast.toasts[0].variant).toBe('warning');
    });

    it('adds an info toast', () => {
        toast.info('Info message');
        expect(toast.toasts).toHaveLength(1);
        expect(toast.toasts[0].title).toBe('Info message');
        expect(toast.toasts[0].variant).toBe('default');
    });

    it('adds multiple toasts in queue', () => {
        toast.success('First');
        toast.error('Second');
        toast.warning('Third');
        expect(toast.toasts).toHaveLength(3);
        expect(toast.toasts[0].title).toBe('First');
        expect(toast.toasts[1].title).toBe('Second');
        expect(toast.toasts[2].title).toBe('Third');
    });

    it('removes toast by id', () => {
        toast.success('First');
        toast.error('Second');
        const toastId = toast.toasts[0].id;

        toast.remove(toastId);
        expect(toast.toasts).toHaveLength(1);
        expect(toast.toasts[0].title).toBe('Second');
    });

    it('clears all toasts', () => {
        toast.success('First');
        toast.error('Second');
        toast.warning('Third');

        toast.clear();
        expect(toast.toasts).toHaveLength(0);
    });

    it('generates unique ids for each toast', () => {
        toast.success('First');
        toast.error('Second');

        const id1 = toast.toasts[0].id;
        const id2 = toast.toasts[1].id;

        expect(id1).not.toBe(id2);
    });

    it('accepts custom description', () => {
        toast.success('Title', 'Description');
        expect(toast.toasts[0].title).toBe('Title');
        expect(toast.toasts[0].description).toBe('Description');
    });

    it('accepts custom action', () => {
        toast.success('Title', 'Description', {
            label: 'Undo',
            onClick: () => {}
        });
        expect(toast.toasts[0].action).toBeDefined();
        expect(toast.toasts[0].action?.label).toBe('Undo');
    });

    it('sets default timeout of 5 seconds', () => {
        toast.success('Title');
        expect(toast.toasts[0].timeout).toBe(5);
    });

    it('accepts custom timeout override', () => {
        toast.success('Title', 'Description', undefined, 5);
        expect(toast.toasts[0].timeout).toBe(5);
    });

    it('accepts custom timeout for error toasts', () => {
        toast.error('Title', 'Description', undefined, 15);
        expect(toast.toasts[0].timeout).toBe(15);
    });

    it('accepts custom timeout for warning toasts', () => {
        toast.warning('Title', 'Description', undefined, 3);
        expect(toast.toasts[0].timeout).toBe(3);
    });

    it('accepts custom timeout for info toasts', () => {
        toast.info('Title', 'Description', undefined, 20);
        expect(toast.toasts[0].timeout).toBe(20);
    });

    it('allows zero timeout to disable auto-dismiss', () => {
        toast.success('Title', 'Description', undefined, 0);
        expect(toast.toasts[0].timeout).toBe(0);
    });
});
