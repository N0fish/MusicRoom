document.addEventListener('DOMContentLoaded', () => {
    const modal = document.getElementById('modal');
    const modalTitle = document.getElementById('modal-title');
    const modalContent = document.getElementById('modal-content');
    const modalInput = document.getElementById('modal-input');
    const modalActions = document.getElementById('modal-actions');
    const modalCloseBtn = document.getElementById('modal-close-btn');

    function showModal() {
        modal.classList.remove('hidden');
    }

    function hideModal() {
        modal.classList.add('hidden');
    }

    if(modalCloseBtn) {
        modalCloseBtn.addEventListener('click', hideModal);
    }


    window.showAlert = ({ title, content }) => {
        modalTitle.textContent = title;
        modalContent.textContent = content;
        modalInput.classList.add('hidden');
        modalActions.innerHTML = '';
        const closeButton = document.createElement('button');
        closeButton.textContent = 'Close';
        closeButton.className = 'px-4 py-2 bg-gray-700 text-text text-base font-medium rounded-md w-full shadow-sm hover:bg-gray-600 focus:outline-none';
        closeButton.addEventListener('click', hideModal);
        modalActions.appendChild(closeButton);
        showModal();
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
        cancelButton.className = 'mt-2 px-4 py-2 bg-gray-700 text-text text-base font-medium rounded-md w-full shadow-sm hover:bg-gray-600 focus:outline-none';
        cancelButton.addEventListener('click', hideModal);

        modalActions.appendChild(confirmButton);
        modalActions.appendChild(cancelButton);

        showModal();
    }
});
