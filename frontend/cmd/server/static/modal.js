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

    window.showAlert = ({ title, content }) => {
        toastTitle.textContent = title;
        toastContent.textContent = content;
        toast.classList.remove('hidden');

        setTimeout(() => {
            toast.classList.add('hidden');
        }, 3000);
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
});
