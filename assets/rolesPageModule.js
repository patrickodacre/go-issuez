import env from './env'
import axios from 'axios'

export default () => {
    const triggers = document.querySelectorAll('[data-role-delete]')

    const $modal = $('[data-issuez-delete-modal="role"]')

    let target = null
    let deleteConfirmBtn = null

    $modal.on('hidden.bs.modal', function (e) {
        deleteConfirmBtn.removeEventListener('click', confirmDelete)
    })

    $modal.on('show.bs.modal', function (e) {
        deleteConfirmBtn = document.querySelector('[data-delete-confirm]')

        const content = document.querySelector('[data-delete-modal-content]')
        const title = document.querySelector('[data-delete-modal-title]')

        title.innerHTML = "Delete " + (target.Name || '')

        content.innerHTML = 'Are you sure?'

        deleteConfirmBtn.addEventListener('click', confirmDelete)
    })

    triggers.forEach(trigger => {
        trigger.addEventListener('click', evt => {

            const rawData = evt.currentTarget.getAttribute('data-role-delete')

            target = rawData
                ? JSON.parse(rawData)
                : {}

            $modal.modal('show')
        })
    })

    function confirmDelete() {

        axios.delete(`${env.APP_URL}/roles/${target.ID}`)
            .then(resp => {
                window.location.href = `${env.APP_URL}/admin/roles`
            })
            .catch(err => {
                alert(err.message)
                console.log(err)
            })
    }
}
