document.addEventListener('DOMContentLoaded', () => {
    const modal = document.getElementById('modal');
    const modalTitle = document.getElementById('modal-title');
    const modalContent = document.getElementById('modal-content');
    const modalInput = document.getElementById('modal-input');
    const modalActions = document.getElementById('modal-actions');

    const toast = document.getElementById('toast');
    const toastTitle = document.getElementById('toast-title');
    const toastContent = document.getElementById('toast-content');

    function showModal() {
        modal.classList.remove('hidden');
    }

    function hideModal() {
        modal.classList.add('hidden');
    }

    window.closeModal = hideModal;

    window.showModal = ({ title, content, buttons }) => {
        modalTitle.textContent = title;
        modalContent.innerHTML = content; // Allow HTML content
        modalInput.classList.add('hidden');
        modalActions.innerHTML = '';

        if (buttons && buttons.length > 0) {
            buttons.forEach(btn => {
                const buttonEl = document.createElement('button');
                buttonEl.textContent = btn.text;
                buttonEl.className = btn.class || 'btn';
                if (btn.onclick) {
                    buttonEl.addEventListener('click', btn.onclick);
                } else {
                    buttonEl.addEventListener('click', hideModal);
                }
                // Add some spacing if not first
                if (modalActions.children.length > 0) {
                    buttonEl.classList.add('ml-2'); 
                }
                modalActions.appendChild(buttonEl);
            });
        } else {
            // Default close button
            const cancelButton = document.createElement('button');
            cancelButton.textContent = 'Close';
            cancelButton.className = 'btn-modal mt-2';
            cancelButton.addEventListener('click', hideModal);
            modalActions.appendChild(cancelButton);
        }

        showModal();
    }

    window.showAlert = ({ title, content }) => {
        toastTitle.textContent = title;
        toastContent.textContent = content;
        toast.classList.remove('hidden');

        setTimeout(() => {
            toast.classList.add('hidden');
        }, 3000);
    }

    window.showToast = (content) => {
        window.showAlert({ title: 'Notification', content: content });
    }

    window.showPrompt = ({ title, content, onConfirm }) => {
        modalTitle.textContent = title;
        modalContent.textContent = content;
        modalInput.classList.remove('hidden');
        modalInput.value = '';
        modalActions.innerHTML = '';

        const confirmButton = document.createElement('button');
        confirmButton.textContent = 'Confirm';
        confirmButton.className = 'px-4 py-2 bg-primary text-white text-base font-medium rounded-md w-full shadow-sm hover:bg-primary-hover focus:outline-none';
        confirmButton.addEventListener('click', () => {
            onConfirm(modalInput.value);
            hideModal();
        });

        const cancelButton = document.createElement('button');
        cancelButton.textContent = 'Cancel';
        cancelButton.className = 'btn-modal mt-2';
        cancelButton.addEventListener('click', hideModal);

        modalActions.appendChild(confirmButton);
        modalActions.appendChild(cancelButton);

        showModal();
    }

    window.showConfirm = ({ title, content, onConfirm }) => {
        modalTitle.textContent = title;
        modalContent.textContent = content;
        modalInput.classList.add('hidden');
        modalActions.innerHTML = '';

        const confirmButton = document.createElement('button');
        confirmButton.textContent = 'Confirm';
        confirmButton.className = 'px-4 py-2 bg-primary text-white text-base font-medium rounded-md w-full shadow-sm hover:bg-primary-hover focus:outline-none';
        confirmButton.addEventListener('click', () => {
            onConfirm();
            hideModal();
        });

        const cancelButton = document.createElement('button');
        cancelButton.textContent = 'Cancel';
        cancelButton.className = 'btn-modal mt-2';
        cancelButton.addEventListener('click', hideModal);

        modalActions.appendChild(confirmButton);
        modalActions.appendChild(cancelButton);

        showModal();
    }
});
