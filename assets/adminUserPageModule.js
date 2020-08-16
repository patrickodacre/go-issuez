import env from './env'
import axios from 'axios'

export default () => {

    const userRoleSelectInputs = document.querySelectorAll('[data-user-role-select]')

    userRoleSelectInputs.forEach(roleSelect => {
        roleSelect.addEventListener('change', evt => {

            const user_id = roleSelect.getAttribute('data-user-id')
            const role_id = evt.currentTarget.value

            axios.post(env.APP_URL + "/admin/setUserRole", {user_id, role_id})
                .then(r => {
                    alert("Role Set")
                })
                .catch(e => {
                    console.error(e)
                    alert("Could not set user role.")
                })
        })
    })

    // delete
    const triggers = document.querySelectorAll('[data-user-delete]')

    const $modal = $('[data-issuez-delete-modal="user"]')

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

            const raw = trigger.getAttribute('data-user-delete')

            target = raw
                ? JSON.parse(raw)
                : {}

            $modal.modal('show')
        })
    })

    function confirmDelete() {

        axios.delete(`${env.APP_URL}/users/${target.ID}`)
            .then(resp => {
                window.location.href = `${env.APP_URL}/admin/users`
            })
            .catch(err => {
                console.log(err)
            })
    }
}
