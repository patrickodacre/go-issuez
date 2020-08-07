import env from './env'
import axios from 'axios'

export default () => {
    const trigger = document.querySelector('[data-bug-delete]')

    const $modal = $('[data-issuez-delete-modal="bug"]')

    let targetBug = null
    let deleteConfirmBtn = null

    $modal.on('hidden.bs.modal', function (e) {
        deleteConfirmBtn.removeEventListener('click', confirmDelete)
    })

    $modal.on('show.bs.modal', function (e) {
        deleteConfirmBtn = document.querySelector('[data-bug-delete-confirm]')

        const content = document.querySelector('[data-delete-modal-content]')
        const title = document.querySelector('[data-delete-modal-title]')

        title.innerHTML = "Delete " + (targetBug.Name || '')

        content.innerHTML = 'Are you sure?'

        deleteConfirmBtn.addEventListener('click', confirmDelete)
    })

    trigger.addEventListener('click', evt => {

        const bugDataRaw = trigger.getAttribute('data-bug-delete')

        targetBug = bugDataRaw
            ? JSON.parse(bugDataRaw)
            : {}

        $modal.modal('show')
    })

    function confirmDelete() {

        axios.delete(`${env.APP_URL}/bugs/${targetBug.ID}`)
            .then(resp => {
                window.location.href = `${env.APP_URL}/features/${targetBug.FeatureID}`
            })
            .catch(err => {
                console.log(err)
            })
    }
}

